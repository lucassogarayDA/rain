package aar

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MergeResources fusiona los res/ de varias librerías en un único directorio,
// resolviendo duplicados igual que AGP: para values*, gana el primero que
// define (config, tipo, nombre); para ficheros no-values, gana el primero con
// la misma ruta relativa. aapt2 no admite dos definiciones del mismo recurso.
func MergeResources(resDirs []string, destDir string) error {
	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	seenValues := make(map[string]bool) // config|tipo|nombre
	seenFiles := make(map[string]bool)  // ruta relativa
	// config -> elementos <resources> acumulados
	acc := make(map[string]*resourcesElement)
	var configs []string

	for _, resDir := range resDirs {
		if resDir == "" {
			continue
		}
		err := filepath.Walk(resDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(resDir, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			config := configFromRel(rel)

			if config == "" {
				if seenFiles[rel] {
					return nil
				}
				seenFiles[rel] = true
				return copyFileInto(path, filepath.Join(destDir, rel))
			}

			root, err := parseResources(path)
			if err != nil {
				// XML no parseable: trátalo como fichero normal.
				if seenFiles[rel] {
					return nil
				}
				seenFiles[rel] = true
				return copyFileInto(path, filepath.Join(destDir, rel))
			}

			out, ok := acc[config]
			if !ok {
				out = &resourcesElement{attrs: root.attrs}
				acc[config] = out
				configs = append(configs, config)
			}
			for _, el := range root.children {
				if el.name == "" {
					out.children = append(out.children, el)
					continue
				}
				key := config + "|" + el.kind + "|" + el.name
				if seenValues[key] {
					continue
				}
				seenValues[key] = true
				out.children = append(out.children, el)
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	sort.Strings(configs)
	for _, config := range configs {
		out := acc[config]
		if len(out.children) == 0 {
			continue
		}
		path := filepath.Join(destDir, config, "merged.xml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := writeResources(path, out); err != nil {
			return err
		}
	}
	return nil
}

// configFromRel devuelve "values", "values-af"... si el recurso es de values.
func configFromRel(rel string) string {
	parts := strings.Split(rel, "/")
	if len(parts) == 0 {
		return ""
	}
	if strings.HasPrefix(parts[0], "values") {
		return parts[0]
	}
	return ""
}

type resourceChild struct {
	name  string // atributo name ("" para elementos sin name)
	kind  string // tipo deducido (string, color, item...)
	raw   []byte // XML serializado del elemento
	attrs map[string]string
}

type resourcesElement struct {
	attrs    map[string]string
	children []resourceChild
}

type parsedResources struct {
	attrs    map[string]string
	children []resourceChild
}

func parseResources(path string) (*parsedResources, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	out := &parsedResources{attrs: map[string]string{}}

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "resources" {
			return nil, fmt.Errorf("raíz no es <resources>: %s", path)
		}
		for _, a := range start.Attr {
			out.attrs[a.Name.Local] = a.Value
		}
		break
	}

	// Recorre los hijos directos de <resources>.
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			if end, ok := tok.(xml.EndElement); ok && end.Name.Local == "resources" {
				break
			}
			continue
		}

		child := resourceChild{attrs: map[string]string{}}
		child.name = attrValue(start.Attr, "name")
		for _, a := range start.Attr {
			child.attrs[a.Name.Local] = a.Value
		}
		child.kind = resourceKind(start.Name.Local, child.attrs)

		var buf strings.Builder
		enc := xml.NewEncoder(&buf)
		if err := enc.EncodeToken(start); err != nil {
			return nil, err
		}
		depth := 1
		for depth > 0 {
			inner, err := dec.Token()
			if err != nil {
				return nil, err
			}
			if err := enc.EncodeToken(inner); err != nil {
				return nil, err
			}
			switch inner.(type) {
			case xml.StartElement:
				depth++
			case xml.EndElement:
				depth--
			}
		}
		if err := enc.Flush(); err != nil {
			return nil, err
		}
		child.raw = []byte(buf.String())
		out.children = append(out.children, child)
	}

	return out, nil
}

func resourceKind(tag string, attrs map[string]string) string {
	if tag == "item" {
		if t := attrs["type"]; t != "" {
			return t
		}
		return "id"
	}
	return tag
}

func attrValue(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func writeResources(path string, res *resourcesElement) error {
	var buf strings.Builder
	buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	buf.WriteString("<resources")
	for k, v := range res.attrs {
		buf.WriteString(fmt.Sprintf(" %s=%q", k, v))
	}
	buf.WriteString(">\n")
	for _, child := range res.children {
		buf.Write(child.raw)
		buf.WriteString("\n")
	}
	buf.WriteString("</resources>\n")
	return os.WriteFile(path, []byte(buf.String()), 0o644)
}

func copyFileInto(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
