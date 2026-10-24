package hilt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Anotaciones que convierten una clase en "entry point" de Hilt. Son las
// mismas que mira el plugin de Gradle (AndroidEntryPointClassVisitor).
var entryPointAnnotations = []string{
	"Ldagger/hilt/android/AndroidEntryPoint;",
	"Ldagger/hilt/android/HiltAndroidApp;",
}

// HiltBaseName devuelve el nombre de la clase base que Hilt genera para una
// clase: Hilt_MainActivity, Hilt_MyApp... Las clases anidadas usan "_" en
// lugar de "$" (Hilt_Foo_Bar), igual que hace el plugin de Gradle.
func HiltBaseName(internalName string) string {
	simple := internalName
	if idx := strings.LastIndex(internalName, "/"); idx >= 0 {
		simple = internalName[idx+1:]
	}
	simple = strings.ReplaceAll(simple, "$", "_")
	pkg := ""
	if idx := strings.LastIndex(internalName, "/"); idx >= 0 {
		pkg = internalName[:idx+1]
	}
	return pkg + "Hilt_" + simple
}

// IsEntryPoint indica si el class file corresponde a una clase anotada con
// @AndroidEntryPoint o @HiltAndroidApp.
func IsEntryPoint(data []byte) (bool, error) {
	cf, err := parseClassFile(data)
	if err != nil {
		return false, err
	}
	for _, descriptor := range entryPointAnnotations {
		if cf.hasClassAnnotation(descriptor) {
			return true, nil
		}
	}
	return false, nil
}

// TransformClass reescribe la superclase de una clase anotada con
// @AndroidEntryPoint o @HiltAndroidApp para que herede de la clase base que
// genera Hilt.
//
// Devuelve el class file reescrito y true si hubo cambios.
//
// Cambia dos cosas, igual que el visitor de ASM del plugin de Gradle:
//
//   - super_class apunta a Hilt_<Clase>.
//   - Los methodrefs que llamaban a la superclase antigua (el invokespecial
//     del constructor y las llamadas a super) apuntan a la nueva. Sin esto la
//     JVM rechaza la clase: el invokespecial <init> de un constructor tiene
//     que apuntar a la superclase directa.
func TransformClass(data []byte) ([]byte, bool, error) {
	cf, err := parseClassFile(data)
	if err != nil {
		return nil, false, err
	}

	entryPoint := false
	for _, descriptor := range entryPointAnnotations {
		if cf.hasClassAnnotation(descriptor) {
			entryPoint = true
			break
		}
	}
	if !entryPoint {
		return data, false, nil
	}

	if cf.superClass == 0 {
		// Object: no hay superclase que cambiar.
		return data, false, nil
	}

	oldSuper := cf.className(cf.superClass)
	if oldSuper == "" {
		return nil, false, fmt.Errorf("no se pudo leer la superclase de la clase")
	}

	ownName := cf.className(cf.thisClass)
	newSuper := HiltBaseName(ownName)
	if newSuper == oldSuper {
		// Ya transformada (o una clase que ya hereda de Hilt_X).
		return data, false, nil
	}

	newIndex := cf.addClassEntry(newSuper)

	// Los methodrefs que apuntaban a la superclase antigua pasan a apuntar a
	// la nueva. Se reescribe el class_index de la entrada, no se crea una
	// entrada nueva: el effecto es el mismo (el invokespecial resuelve por
	// el índice de la clase) y es más simple.
	oldIndex := cf.superClass
	for i := range cf.pool {
		entry := &cf.pool[i]
		if entry.tag != tagMethodref && entry.tag != tagInterfaceMethodref {
			continue
		}
		if entry.refIndex == oldIndex {
			entry.data[1] = byte(newIndex >> 8)
			entry.data[2] = byte(newIndex)
			entry.refIndex = newIndex
		}
	}

	cf.superClass = newIndex
	return cf.serialize(), true, nil
}

// TransformDir reescribe en el sitio todas las clases anotadas de un
// directorio de clases compiladas. Devuelve cuántas se han tocado.
func TransformDir(dir string) (int, error) {
	count := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".class") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out, changed, err := TransformClass(data)
		if err != nil {
			// Una clase que no se puede parsear no es culpa de Hilt: se
			// avisa y se sigue, igual que hace el plugin de Gradle (que
			// ignora las clases que no puede instrumentar).
			fmt.Fprintf(os.Stderr, "aviso: no se pudo transformar %s: %v\n", path, err)
			return nil
		}
		if !changed {
			return nil
		}
		if err := os.WriteFile(path, out, info.Mode().Perm()); err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		return count, err
	}
	return count, nil
}
