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

// androidNamespace es el namespace de los atributos android: de un manifest.
const androidNamespace = "http://schemas.android.com/apk/res/android"

// MergeManifests fusiona los AndroidManifest.xml de las librerías (AAR) en el
// manifest de la aplicación.
//
// La fusión es textual a propósito: se conserva el fragmento original de cada
// librería, con sus prefijos de namespace intactos. Re-serializar el XML
// (encoding/xml) pierde los prefijos —un android:versionCode se convertiría
// en un versionCode sin namespace— y el manifest resultante lo rechaza aapt2.
//
// Fusiona:
//   - <uses-permission> (sin duplicados por android:name)
//   - los componentes declarados dentro de <application>: <activity>,
//     <activity-alias>, <service>, <receiver>, <provider> y <meta-data>
//     (sin duplicados por etiqueta y android:name)
//
// Queda fuera, y no se pretende cubrir: <uses-sdk> (lo decide la app),
// <uses-feature>, <queries> y los atributos de <application>, que en AndroidX
// suelen traer valores por defecto que no aplican a la app.
func MergeManifests(mainManifest string, libraryManifests []string, outputPath string) error {
	mainData, err := os.ReadFile(mainManifest)
	if err != nil {
		return fmt.Errorf("leyendo el manifest principal: %w", err)
	}
	mainText := string(mainData)

	mainPerms, mainComps, err := scanManifest(mainText)
	if err != nil {
		return fmt.Errorf("analizando el manifest principal: %w", err)
	}

	havePerms := make(map[string]bool, len(mainPerms))
	for _, perm := range mainPerms {
		havePerms[perm.key()] = true
	}
	haveComps := make(map[string]bool, len(mainComps))
	for _, comp := range mainComps {
		haveComps[comp.key()] = true
	}

	var newPerms, newComps []manifestSnippet
	// Los fragmentos copiados pueden usar prefijos que el manifest de la app no
	// declara (típicamente tools:node="merge" de AndroidX Startup). Si el
	// namespace no está declarado en el documento fusionado, aapt2 lo rechaza
	// con "unbound prefix". Se recogen las declaraciones de cada librería.
	namespaces := make(map[string]string)
	for prefix, uri := range rootNamespaces(mainText) {
		namespaces[prefix] = uri
	}
	mainNamespaces := make(map[string]string, len(namespaces))
	for prefix, uri := range namespaces {
		mainNamespaces[prefix] = uri
	}

	for _, libPath := range libraryManifests {
		if libPath == "" {
			continue
		}
		libData, err := os.ReadFile(libPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("leyendo el manifest de la librería %s: %w", libPath, err)
		}
		libText := string(libData)
		for prefix, uri := range rootNamespaces(libText) {
			if _, ok := namespaces[prefix]; !ok {
				namespaces[prefix] = uri
			}
		}
		perms, comps, err := scanManifest(libText)
		if err != nil {
			// Un manifest de librería inválido no debe abortar la fusión.
			continue
		}
		for _, perm := range perms {
			if perm.name == "" || havePerms[perm.key()] {
				continue
			}
			havePerms[perm.key()] = true
			newPerms = append(newPerms, perm)
		}
		for _, comp := range comps {
			if haveComps[comp.key()] {
				continue
			}
			haveComps[comp.key()] = true
			newComps = append(newComps, comp)
		}
	}

	merged, err := applyManifestSnippets(mainText, newPerms, newComps)
	if err != nil {
		return err
	}
	// Se declaran en el raíz los namespaces que usan los fragmentos copiados y
	// que el manifest de la app no declaraba.
	missing := make(map[string]string)
	for prefix, uri := range namespaces {
		if _, ok := mainNamespaces[prefix]; !ok {
			missing[prefix] = uri
		}
	}
	if len(missing) > 0 {
		merged, err = declareNamespaces(merged, missing)
		if err != nil {
			return err
		}
	}
	// Red de seguridad: si el manifest fusionado no es XML válido, es mejor
	// fallar aquí que dejar que aapt2 falle con un error incomprensible.
	if err := validateManifest(merged); err != nil {
		return fmt.Errorf("el manifest fusionado no es válido: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, []byte(merged), 0o644)
}

// manifestSnippet es un elemento del manifest con su texto original.
type manifestSnippet struct {
	tag  string
	name string
	raw  string
}

// key identifica el elemento para deduplicarlo: etiqueta + android:name.
func (s manifestSnippet) key() string {
	if s.name == "" {
		return s.tag
	}
	return s.tag + ":" + s.name
}

// scanManifest devuelve los <uses-permission> y los componentes de
// <application> de un manifest, con el texto original de cada elemento.
func scanManifest(text string) (perms, comps []manifestSnippet, err error) {
	dec := xml.NewDecoder(strings.NewReader(text))
	inApp := false

	for {
		before := dec.InputOffset()
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		switch node := tok.(type) {
		case xml.EndElement:
			if node.Name.Local == "application" {
				inApp = false
			}
			continue
		case xml.StartElement:
			switch node.Name.Local {
			case "manifest":
				continue
			case "application":
				inApp = true
				continue
			}
		default:
			continue
		}

		start := tok.(xml.StartElement)
		raw, err := readRawElement(dec, text, before)
		if err != nil {
			return nil, nil, err
		}
		snippet := manifestSnippet{
			tag:  start.Name.Local,
			name: androidAttr(start.Attr, "name"),
			raw:  trimSnippet(raw, start.Name.Local),
		}

		switch {
		case inApp && isComponentTag(start.Name.Local):
			comps = append(comps, snippet)
		case !inApp && start.Name.Local == "uses-permission":
			perms = append(perms, snippet)
		}
	}

	return perms, comps, nil
}

// readRawElement lee el elemento actual (el decoder está posicionado en su
// StartElement) y devuelve el texto original del documento, con sus
// namespaces y su formato intactos.
func readRawElement(dec *xml.Decoder, text string, startOffset int64) (string, error) {
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	end := dec.InputOffset()
	if startOffset < 0 || end > int64(len(text)) || startOffset >= end {
		return "", fmt.Errorf("rango inválido (%d-%d) al leer un elemento del manifest", startOffset, end)
	}
	return text[startOffset:end], nil
}

// trimSnippet recorta el fragmento para que empiece justo en la etiqueta y no
// arrastre el espacio en blanco anterior.
func trimSnippet(raw, tag string) string {
	if idx := strings.Index(raw, "<"+tag); idx > 0 {
		raw = raw[idx:]
	}
	return strings.TrimRight(raw, " \t\r\n")
}

// applyManifestSnippets inserta los fragmentos en el manifest de la app: los
// permisos antes de <application> y los componentes dentro de él.
func applyManifestSnippets(main string, perms, comps []manifestSnippet) (string, error) {
	out := main

	if len(perms) > 0 {
		idx := strings.Index(out, "<application")
		if idx < 0 {
			idx = strings.Index(out, "</manifest>")
		}
		if idx < 0 {
			return "", fmt.Errorf("el manifest no parece válido: no hay <application> ni </manifest>")
		}
		out = out[:idx] + renderSnippets(perms) + out[idx:]
	}

	if len(comps) > 0 {
		closeIdx := strings.LastIndex(out, "</application>")
		if closeIdx >= 0 {
			out = out[:closeIdx] + renderSnippets(comps) + out[closeIdx:]
			return out, nil
		}
		// <application/> auto-cerrado: se convierte en un elemento con
		// contenido para poder insertar los componentes.
		openIdx := strings.Index(out, "<application")
		if openIdx < 0 {
			return "", fmt.Errorf("el manifest no parece válido: no hay <application>")
		}
		end := strings.Index(out[openIdx:], ">")
		if end < 0 {
			return "", fmt.Errorf("el manifest no parece válido: <application> sin cerrar")
		}
		end += openIdx
		tag := strings.TrimSpace(out[openIdx : end+1])
		if !strings.HasSuffix(tag, "/>") {
			return "", fmt.Errorf("el manifest no parece válido: <application> sin cerrar")
		}
		tag = strings.TrimSuffix(tag, "/>") + ">"
		out = out[:openIdx] + tag + "\n" + renderSnippets(comps) + "</application>" + out[end+1:]
	}

	return out, nil
}

// renderSnippets concatena los fragmentos, uno por línea.
func renderSnippets(snippets []manifestSnippet) string {
	var buf strings.Builder
	for _, snippet := range snippets {
		buf.WriteString("    ")
		buf.WriteString(snippet.raw)
		buf.WriteString("\n")
	}
	return buf.String()
}

// rootNamespaces devuelve las declaraciones xmlns:* del elemento raíz del
// manifest (prefijo -> URI). Los fragmentos copiados de las librerías pueden
// usar esos prefijos, y si no se declaran en el documento fusionado aapt2
// responde "unbound prefix".
func rootNamespaces(text string) map[string]string {
	dec := xml.NewDecoder(strings.NewReader(text))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		ns := make(map[string]string)
		for _, a := range start.Attr {
			if a.Name.Space == "xmlns" {
				ns[a.Name.Local] = a.Value
			}
		}
		return ns
	}
}

// declareNamespaces añade las declaraciones xmlns:... al elemento raíz del
// manifest fusionado.
func declareNamespaces(text string, ns map[string]string) (string, error) {
	idx := strings.Index(text, "<manifest")
	if idx < 0 {
		return "", fmt.Errorf("el manifest no parece válido: no hay <manifest>")
	}
	end := strings.Index(text[idx:], ">")
	if end < 0 {
		return "", fmt.Errorf("el manifest no parece válido: <manifest> sin cerrar")
	}
	end += idx

	prefixes := make([]string, 0, len(ns))
	for prefix := range ns {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	var decls strings.Builder
	for _, prefix := range prefixes {
		decls.WriteString(fmt.Sprintf("\n    xmlns:%s=%q", prefix, ns[prefix]))
	}

	// Caso improbable de un raíz auto-cerrado: hay que abrirlo para poder
	// declarar los namespaces.
	if strings.HasSuffix(strings.TrimSpace(text[idx:end]), "/") {
		tag := strings.TrimSuffix(strings.TrimSpace(text[idx:end]), "/")
		return text[:idx] + tag + ">" + decls.String() + "</manifest>" + text[end+1:], nil
	}
	return text[:end] + decls.String() + text[end:], nil
}

// isComponentTag indica si la etiqueta es un componente declarable dentro de
// <application>.
func isComponentTag(tag string) bool {
	switch tag {
	case "activity", "activity-alias", "service", "receiver", "provider", "meta-data":
		return true
	}
	return false
}

// postNotificationsPermission es el permiso que AGP inyecta por su cuenta a
// partir de API 33.
const postNotificationsPermission = "android.permission.POST_NOTIFICATIONS"

// WithImplicitPermissions devuelve el manifest con los permisos que el merger
// de AGP añade implícitamente según el targetSdk.
//
// Hoy solo POST_NOTIFICATIONS: desde Android 13 (API 33) una app necesita
// declararlo para poder publicar notificaciones. AGP lo inyecta solo, así que
// una app que no lo declara en su manifest funciona con Gradle y con Rain no:
// el sistema descarta las notificaciones en silencio y la app parece no hacer
// nada. Rain lo añade igual que AGP para que ambos builds se comporten igual.
func WithImplicitPermissions(manifestText string, targetSdk int) (string, error) {
	if targetSdk < 33 {
		return manifestText, nil
	}
	if strings.Contains(manifestText, postNotificationsPermission) {
		return manifestText, nil
	}

	idx := strings.Index(manifestText, "<manifest")
	if idx < 0 {
		return manifestText, fmt.Errorf("el manifest no parece válido: no se encuentra <manifest>")
	}
	// Se busca el cierre de la etiqueta de apertura respetando las comillas
	// de los atributos: un ">" dentro de un valor no cierra la etiqueta.
	end := -1
	inQuotes := false
	for i := idx; i < len(manifestText); i++ {
		switch manifestText[i] {
		case '"':
			inQuotes = !inQuotes
		case '>':
			if !inQuotes {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return manifestText, fmt.Errorf("el manifest no parece válido: <manifest> sin cerrar")
	}

	perm := fmt.Sprintf("\n    <uses-permission android:name=%q />", postNotificationsPermission)
	return manifestText[:end+1] + perm + manifestText[end+1:], nil
}

// androidAttr devuelve el valor de un atributo, tenga o no el prefijo
// android: (el namespace por defecto de un manifest).
func androidAttr(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local != local {
			continue
		}
		if a.Name.Space == "" || a.Name.Space == androidNamespace {
			return a.Value
		}
	}
	return ""
}

// validateManifest comprueba que el texto sea un XML bien formado.
func validateManifest(text string) error {
	dec := xml.NewDecoder(strings.NewReader(text))
	for {
		_, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
