// Package hilt implementa la transformación de bytecode que hace el plugin
// de Gradle de Hilt sobre las clases anotadas con @AndroidEntryPoint o
// @HiltAndroidApp.
//
// El plugin de Gradle no toca el código fuente: compila la app tal cual y
// después, con ASM, cambia la superclase de cada clase anotada por la clase
// base que genera el processor (Hilt_MainActivity, Hilt_MyApp...). Esa clase
// base es la que hace la inyección de dependencias en runtime, así que sin
// este cambio la app compila pero peta al usar un @Inject.
//
// Aquí se hace lo mismo sin ASM: se reescribe el class file directamente.
// Solo hacen falta dos cambios:
//
//  1. super_class pasa a apuntar a la clase base de Hilt.
//  2. Los methodrefs que llamaban a la superclase antigua (los invokespecial
//     del constructor y de las llamadas a super) pasan a apuntar a la nueva.
//
// El punto 2 es obligatorio: la JVM exige que el invokespecial <init> de un
// constructor apunte a la superclase directa, si no el verificador falla.
package hilt

import (
	"encoding/binary"
	"fmt"
)

// Tags del constant pool (JVM spec, §4.4).
const (
	tagUtf8               = 1
	tagInteger            = 3
	tagFloat              = 4
	tagLong               = 5
	tagDouble             = 6
	tagClass              = 7
	tagString             = 8
	tagFieldref           = 9
	tagMethodref          = 10
	tagInterfaceMethodref = 11
	tagNameAndType        = 12
	tagMethodHandle       = 15
	tagMethodType         = 16
	tagDynamic            = 17
	tagInvokeDynamic      = 18
	tagModule             = 19
	tagPackage            = 20
)

// poolEntry es una entrada del constant pool.
//
// data guarda los bytes tal cual (tag incluido) para poder reescribir el
// pool sin reinterpretar cada tipo. refIndex apunta al índice del
// CONSTANT_Class referenciado (para Class, Fieldref, Methodref e
// InterfaceMethodref) o al name_index (para Utf8).
type poolEntry struct {
	tag      byte
	data     []byte
	refIndex uint16
}

// classFile es un class file Java parseado lo justo para poder reescribir la
// superclase y leer las anotaciones de nivel de clase.
type classFile struct {
	minor uint16
	major uint16
	pool  []poolEntry
	// poolCount es el constant_pool_count original (los índices empiezan en 1).
	poolCount int
	// thisClass y superClass son índices dentro del constant pool.
	thisClass  uint16
	superClass uint16
	// classAttrs guarda los atributos de nivel de clase ya parseados.
	classAttrs []attribute
	// middle son los bytes que van entre el final del constant pool y el
	// final de los atributos de clase: access_flags, this_class, super_class,
	// interfaces, fields y methods. El super_class está en middle[4:6].
	middle []byte
}

// attribute es un attribute_info del class file.
type attribute struct {
	name  string
	index uint16
	data  []byte
}

// parseClassFile parsea un class file.
func parseClassFile(data []byte) (*classFile, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("class file demasiado corto (%d bytes)", len(data))
	}
	if binary.BigEndian.Uint32(data[0:4]) != 0xCAFEBABE {
		return nil, fmt.Errorf("no es un class file (firma incorrecta)")
	}

	cf := &classFile{
		minor: binary.BigEndian.Uint16(data[4:6]),
		major: binary.BigEndian.Uint16(data[6:8]),
	}

	pos := 8
	cf.poolCount = int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2

	// El pool se indexa desde 1; la entrada 0 no existe.
	cf.pool = make([]poolEntry, cf.poolCount)
	for i := 1; i < cf.poolCount; i++ {
		if pos >= len(data) {
			return nil, fmt.Errorf("constant pool truncado")
		}
		tag := data[pos]
		start := pos
		pos++

		var refIndex uint16
		switch tag {
		case tagUtf8:
			if pos+2 > len(data) {
				return nil, fmt.Errorf("CONSTANT_Utf8 truncado")
			}
			length := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			pos += 2 + length
		case tagInteger, tagFloat:
			pos += 4
		case tagLong, tagDouble:
			// Long y Double ocupan dos entradas del pool.
			pos += 8
		case tagClass, tagString, tagMethodType, tagModule, tagPackage:
			if pos+2 > len(data) {
				return nil, fmt.Errorf("entrada del constant pool truncada")
			}
			refIndex = binary.BigEndian.Uint16(data[pos : pos+2])
			pos += 2
		case tagFieldref, tagMethodref, tagInterfaceMethodref:
			if pos+4 > len(data) {
				return nil, fmt.Errorf("entrada del constant pool truncada")
			}
			refIndex = binary.BigEndian.Uint16(data[pos : pos+2])
			pos += 4
		case tagNameAndType:
			pos += 4
		case tagMethodHandle:
			pos += 3
		case tagDynamic, tagInvokeDynamic:
			pos += 4
		default:
			return nil, fmt.Errorf("tag desconocido en el constant pool: %d", tag)
		}
		if pos > len(data) {
			return nil, fmt.Errorf("constant pool truncado")
		}

		cf.pool[i] = poolEntry{tag: tag, data: data[start:pos], refIndex: refIndex}

		if tag == tagLong || tag == tagDouble {
			i++ // la segunda ranura queda como hueco
		}
	}

	if pos+6 > len(data) {
		return nil, fmt.Errorf("cabecera de clase truncada")
	}
	midStart := pos
	cf.thisClass = binary.BigEndian.Uint16(data[pos+2 : pos+4])
	cf.superClass = binary.BigEndian.Uint16(data[pos+4 : pos+6])
	pos += 6 // access_flags, this_class, super_class

	// interfaces
	if pos+2 > len(data) {
		return nil, fmt.Errorf("interfaces truncadas")
	}
	interfaces := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2 + 2*interfaces

	// fields y methods: hay que saltarlos, pero cada uno lleva sus propios
	// atributos (con longitud) que hay que leer para saber dónde acaba.
	for _, kind := range []string{"fields", "methods"} {
		if pos+2 > len(data) {
			return nil, fmt.Errorf("%s truncados", kind)
		}
		count := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2
		for i := 0; i < count; i++ {
			if pos+6 > len(data) {
				return nil, fmt.Errorf("%s[%d] truncado", kind, i)
			}
			pos += 6 // access_flags, name_index, descriptor_index
			attrs, next, err := skipAttributes(cf, data, pos)
			if err != nil {
				return nil, err
			}
			_ = attrs
			pos = next
		}
	}

	// Atributos de nivel de clase: aquí viven las anotaciones.
	if pos+2 > len(data) {
		return nil, fmt.Errorf("atributos de clase truncados")
	}
	count := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	for i := 0; i < count; i++ {
		if pos+6 > len(data) {
			return nil, fmt.Errorf("atributo de clase %d truncado", i)
		}
		nameIndex := binary.BigEndian.Uint16(data[pos : pos+2])
		length := int(binary.BigEndian.Uint32(data[pos+2 : pos+6]))
		pos += 6
		if pos+length > len(data) {
			return nil, fmt.Errorf("atributo de clase %d truncado", i)
		}
		cf.classAttrs = append(cf.classAttrs, attribute{
			name:  cf.utf8(nameIndex),
			index: nameIndex,
			data:  data[pos : pos+length],
		})
		pos += length
	}

	cf.middle = data[midStart:pos]
	return cf, nil
}

// skipAttributes salta los atributos de un field_info o method_info y
// devuelve la posición siguiente.
func skipAttributes(cf *classFile, data []byte, pos int) ([]attribute, int, error) {
	if pos+2 > len(data) {
		return nil, 0, fmt.Errorf("atributos truncados")
	}
	count := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	attrs := make([]attribute, 0, count)
	for i := 0; i < count; i++ {
		if pos+6 > len(data) {
			return nil, 0, fmt.Errorf("atributo %d truncado", i)
		}
		nameIndex := binary.BigEndian.Uint16(data[pos : pos+2])
		length := int(binary.BigEndian.Uint32(data[pos+2 : pos+6]))
		pos += 6
		if pos+length > len(data) {
			return nil, 0, fmt.Errorf("atributo %d truncado", i)
		}
		attrs = append(attrs, attribute{name: cf.utf8(nameIndex), index: nameIndex, data: data[pos : pos+length]})
		pos += length
	}
	return attrs, pos, nil
}

// utf8 devuelve el texto de una entrada CONSTANT_Utf8.
func (cf *classFile) utf8(index uint16) string {
	if index == 0 || int(index) >= len(cf.pool) {
		return ""
	}
	entry := cf.pool[index]
	if entry.tag != tagUtf8 {
		return ""
	}
	if len(entry.data) < 3 {
		return ""
	}
	length := int(binary.BigEndian.Uint16(entry.data[1:3]))
	if 3+length > len(entry.data) {
		return ""
	}
	return string(entry.data[3 : 3+length])
}

// className devuelve el nombre interno de una entrada CONSTANT_Class
// ("com/ejemplo/Actividad").
func (cf *classFile) className(index uint16) string {
	if index == 0 || int(index) >= len(cf.pool) {
		return ""
	}
	entry := cf.pool[index]
	if entry.tag != tagClass {
		return ""
	}
	return cf.utf8(entry.refIndex)
}

// hasClassAnnotation indica si la clase lleva la anotación indicada.
//
// Se miran los atributos RuntimeVisibleAnnotations y
// RuntimeInvisibleAnnotations: Hilt usa retención CLASS, así que la
// anotación no se ve por reflexión pero sí está en el class file.
func (cf *classFile) hasClassAnnotation(descriptor string) bool {
	for _, attr := range cf.classAttrs {
		switch attr.name {
		case "RuntimeVisibleAnnotations", "RuntimeInvisibleAnnotations":
		default:
			continue
		}
		if cf.annotationsContain(attr.data, descriptor) {
			return true
		}
	}
	return false
}

// annotationsContain busca un descriptor de anotación entre las anotaciones
// de un atributo Runtime[In]VisibleAnnotations.
func (cf *classFile) annotationsContain(data []byte, descriptor string) bool {
	if len(data) < 2 {
		return false
	}
	count := int(binary.BigEndian.Uint16(data[0:2]))
	pos := 2
	for i := 0; i < count; i++ {
		if pos+2 > len(data) {
			return false
		}
		typeIndex := binary.BigEndian.Uint16(data[pos : pos+2])
		pos += 2
		if cf.utf8(typeIndex) == descriptor {
			return true
		}
		// num_element_value_pairs y los pares: hay que saltarlos para
		// llegar a la siguiente anotación.
		if pos+2 > len(data) {
			return false
		}
		pairs := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2
		for j := 0; j < pairs; j++ {
			if pos+2 > len(data) {
				return false
			}
			pos += 2 // element_name_index
			next, ok := skipElementValue(data, pos)
			if !ok {
				return false
			}
			pos = next
		}
	}
	return false
}

// skipElementValue salta un element_value y devuelve la posición siguiente.
func skipElementValue(data []byte, pos int) (int, bool) {
	if pos >= len(data) {
		return 0, false
	}
	tag := data[pos]
	pos++
	switch tag {
	case 'B', 'C', 'D', 'F', 'I', 'J', 'S', 'Z', 's', 'c':
		return pos + 2, pos+2 <= len(data)
	case 'e':
		return pos + 4, pos+4 <= len(data)
	case '@':
		// annotation_value: type_index + num_pairs + pares
		if pos+2 > len(data) {
			return 0, false
		}
		pairs := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		pos += 4
		for i := 0; i < pairs; i++ {
			if pos+2 > len(data) {
				return 0, false
			}
			pos += 2
			next, ok := skipElementValue(data, pos)
			if !ok {
				return 0, false
			}
			pos = next
		}
		return pos, true
	case '[':
		if pos+2 > len(data) {
			return 0, false
		}
		values := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2
		for i := 0; i < values; i++ {
			next, ok := skipElementValue(data, pos)
			if !ok {
				return 0, false
			}
			pos = next
		}
		return pos, true
	default:
		return 0, false
	}
}

// serialize reconstruye el class file.
func (cf *classFile) serialize() []byte {
	poolSize := 0
	for _, entry := range cf.pool {
		poolSize += len(entry.data)
	}
	out := make([]byte, 0, len(cf.pool)*8+64+poolSize+len(cf.middle))

	out = append(out, 0xCA, 0xFE, 0xBA, 0xBE)
	out = binary.BigEndian.AppendUint16(out, cf.minor)
	out = binary.BigEndian.AppendUint16(out, cf.major)
	out = binary.BigEndian.AppendUint16(out, uint16(cf.poolCount))
	for _, entry := range cf.pool {
		out = append(out, entry.data...)
	}
	// middle lleva access_flags, this_class y super_class en los primeros 6
	// bytes: hay que refrescar el super_class con el nuevo índice.
	middle := make([]byte, len(cf.middle))
	copy(middle, cf.middle)
	if len(middle) >= 6 {
		middle[4] = byte(cf.superClass >> 8)
		middle[5] = byte(cf.superClass)
	}
	out = append(out, middle...)
	return out
}

// addClassEntry busca un CONSTANT_Class con ese nombre y, si no existe, lo
// añade al final del pool. Devuelve su índice.
func (cf *classFile) addClassEntry(name string) uint16 {
	for i := 1; i < len(cf.pool); i++ {
		entry := cf.pool[i]
		if entry.tag != tagClass {
			continue
		}
		if cf.utf8(entry.refIndex) == name {
			return uint16(i)
		}
	}
	// Nuevo CONSTANT_Class: hace falta un CONSTANT_Utf8 con el nombre.
	utfIndex := cf.addUtf8(name)
	entry := poolEntry{
		tag:      tagClass,
		data:     append([]byte{tagClass}, binary.BigEndian.AppendUint16(nil, utfIndex)...),
		refIndex: utfIndex,
	}
	cf.pool = append(cf.pool, entry)
	cf.poolCount++
	return uint16(len(cf.pool) - 1)
}

// addUtf8 añade un CONSTANT_Utf8 (reutilizando el existente si ya está).
func (cf *classFile) addUtf8(value string) uint16 {
	for i := 1; i < len(cf.pool); i++ {
		entry := cf.pool[i]
		if entry.tag != tagUtf8 {
			continue
		}
		if len(entry.data) >= 3 {
			length := int(binary.BigEndian.Uint16(entry.data[1:3]))
			if 3+length <= len(entry.data) && string(entry.data[3:3+length]) == value {
				return uint16(i)
			}
		}
	}
	data := append([]byte{tagUtf8}, binary.BigEndian.AppendUint16(nil, uint16(len(value)))...)
	data = append(data, value...)
	cf.pool = append(cf.pool, poolEntry{tag: tagUtf8, data: data})
	cf.poolCount++
	return uint16(len(cf.pool) - 1)
}
