// Package memenv calcula los límites de memoria que Rain aplica a los procesos
// que lanza y detecta entornos con poca RAM.
//
// El problema típico en contenedores (Termux, Alpine, Docker en Android) es que
// la JVM de kotlinc elige por defecto un heap máximo de 1/4 de la RAM
// **física**, no de la disponible. En un contenedor con 2 GB libres y 12 GB de
// máquina, kotlinc reserva 3 GB y el proceso acaba muerto por OOM.
//
// Rain mide la memoria realmente disponible (límite de cgroup si existe, o
// MemAvailable de /proc/meminfo) y acota el heap de la JVM al 50% de ese
// presupuesto, con topes y mínimos seguros. Todo es configurable por entorno:
//
//	RAIN_JAVA_MAX_HEAP=1g   heap máximo de las JVM que lanza Rain
//	RAIN_JAVA_OPTS="..."    opciones extra de la JVM (p. ej. -XX:+UseSerialGC)
//	RAIN_GOMEMLIMIT=1GiB    límite blando del heap del propio runtime de Go
//	RAIN_GOGC=15            porcentaje de GC del runtime de Go
//	GOMEMLIMIT=1GiB         variable estándar de Go (tiene prioridad)
//	GOGC=50                 porcentaje del GC de Go (estándar)
//
// Las dos últimas las lee el propio runtime de Go al arrancar, así que Rain
// las respeta y no las pisa: si están puestas, no aplica ningún límite
// automático.
package memenv

import (
	"bufio"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

const (
	// EnvJavaMaxHeap fija el heap máximo de las JVM que lanza Rain.
	EnvJavaMaxHeap = "RAIN_JAVA_MAX_HEAP"
	// EnvJavaOpts añade opciones a las JVM que lanza Rain.
	EnvJavaOpts = "RAIN_JAVA_OPTS"
	// EnvGoMemLimit fija el límite blando de memoria del runtime de Go.
	EnvGoMemLimit = "RAIN_GOMEMLIMIT"
	// EnvGoGCPercent fija el porcentaje de GC del runtime de Go.
	EnvGoGCPercent = "RAIN_GOGC"
)

// tightGCPercent es el porcentaje de GC que se aplica cuando la memoria
// disponible es escasa. Es agresivo a propósito: en contenedores pequeños
// preferimos gastar CPU antes que morir por OOM.
const tightGCPercent = 15

// maxAutoGoLimit es el tope del límite blando que Rain aplica solo (sin que
// el usuario lo pida). 1 GiB sobra de sobra para un proceso de build.
const maxAutoGoLimit = 1 << 30

// prootGCPercent y prootGoLimit son los valores que mantienen estable al
// runtime de Go cuando el proceso corre bajo proot.
//
// Bajo proot (proot-distro en Termux, por ejemplo) las llamadas al sistema
// se interceptan con ptrace y el runtime de Go se rompe con dos fallos
// característicos, ambos dentro del propio runtime y ninguno caused por Rain:
//
//	fatal error: found pointer to free object
//	SIGSEGV: segmentation violation
//	    runtime.findRunnable
//	    runtime.(*m).becomeSpinning
//
// La diferencia entre funcionar y romperse no es la cantidad de RAM (con
// 3,6 GB libres y el proceso de Rain usando 39 MB también se rompe), sino el
// scavenger: al devolver páginas con madvise(MADV_DONTNEED) el runtime cree
// que las ha liberado y proot no se lo juega bien.
//
// La combinación GOGC=15 + GOMEMLIMIT=1200MiB lo evita: el GC collects muy a
// menudo, el heap se queda en unos pocos megabytes y el scavenger casi no
// llega a madvisear nada. Es la configuración que Rain aplica por defecto
// cuando detecta proot.
const (
	prootGCPercent = 15
	prootGoLimit   = 1200 << 20
)

// DefaultKotlincHeap es el techo del heap de kotlinc cuando la máquina tiene
// memoria de sobra. Kotlin con el plugin de Compose puede pedir bastante, así
// que no conviene bajar de 2g sin motivo.
const DefaultKotlincHeap = "2g"

// minHeapBytes es el mínimo que se permite: por debajo la JVM ni puede
// arrancar Kotlin.
const minHeapBytes = 256 << 20

// heapGranularity redondea los límites a múltiplos de 128 MiB para que los
// valores sean legibles y estables entre ejecuciones.
const heapGranularity = 128 << 20

// Info resume la memoria visible para el proceso.
type Info struct {
	// TotalBytes es la RAM total del sistema (MemTotal).
	TotalBytes int64
	// AvailableBytes es la RAM disponible sin swap (MemAvailable).
	AvailableBytes int64
	// CgroupLimitBytes es el límite duro del contenedor, 0 si no hay cgroup.
	CgroupLimitBytes int64
	// CgroupUsedBytes es el consumo actual dentro del cgroup.
	CgroupUsedBytes int64
}

// HasCgroupLimit indica si el proceso vive dentro de un contenedor con límite
// de memoria conocido.
func (i Info) HasCgroupLimit() bool { return i.CgroupLimitBytes > 0 }

// Budget es la memoria que Rain puede usar sin fastidiar la máquina: lo que
// queda del límite de cgroup o, si no hay cgroup, la RAM disponible.
func (i Info) Budget() int64 {
	if i.CgroupLimitBytes > 0 {
		free := i.CgroupLimitBytes - i.CgroupUsedBytes
		if free > 0 {
			return free
		}
		return 0
	}
	if i.AvailableBytes > 0 {
		return i.AvailableBytes
	}
	return i.TotalBytes
}

// Tight indica si el presupuesto de memoria es escaso: menos de 1 GiB libre o
// menos de la cuarta parte de la RAM total (contenedor limitado dentro de una
// máquina grande).
func (i Info) Tight() bool {
	budget := i.Budget()
	if budget <= 0 {
		return false
	}
	if budget < 1<<30 {
		return true
	}
	return i.TotalBytes > 0 && budget*4 < i.TotalBytes
}

// Detect lee la memoria disponible del sistema o del contenedor.
func Detect() Info {
	info := Info{}
	info.TotalBytes, info.AvailableBytes = readMemInfo()

	// cgroup v2 (unified).
	if limit, ok := readInt64File("/sys/fs/cgroup/memory.max"); ok && isSaneLimit(limit) {
		info.CgroupLimitBytes = limit
		if used, ok := readInt64File("/sys/fs/cgroup/memory.current"); ok {
			info.CgroupUsedBytes = used
		}
		return info
	}

	// cgroup v1.
	if limit, ok := readInt64File("/sys/fs/cgroup/memory/memory.limit_in_bytes"); ok && isSaneLimit(limit) {
		info.CgroupLimitBytes = limit
		if used, ok := readInt64File("/sys/fs/cgroup/memory/memory.usage_in_bytes"); ok {
			info.CgroupUsedBytes = used
		}
	}

	return info
}

// isSaneLimit descarta el "sin límite" de cgroup v1, que se expresa como un
// número enorme en vez de un valor pequeño.
func isSaneLimit(limit int64) bool {
	return limit > 0 && limit < (1<<62)
}

// HeapLimit devuelve el valor de -Xmx que Rain debe usar para las JVM que
// lanza. Respeta RAIN_JAVA_MAX_HEAP y, si no está, calcula un valor a partir
// de la memoria disponible sin bajar de 256 MiB ni superar el tope indicado.
//
// fallback es el valor a usar cuando no se puede medir la memoria (por
// ejemplo, "1g" para d8 o "2g" para kotlinc).
func HeapLimit(fallback string) string {
	if v := strings.TrimSpace(os.Getenv(EnvJavaMaxHeap)); v != "" {
		return v
	}

	fallbackBytes, ok := ParseSize(fallback)
	if !ok || fallbackBytes <= 0 {
		fallbackBytes = 2 << 30
	}

	budget := Detect().Budget()
	if budget <= 0 {
		return fallback
	}

	heap := budget / 2
	if heap > fallbackBytes {
		heap = fallbackBytes
	}
	if heap < minHeapBytes {
		heap = minHeapBytes
	}
	return FormatSize(heap)
}

// UserHeapOverride indica si el usuario ya fijó un heap en alguna variable de
// entorno estándar. En ese caso Rain no debe inyectar su propio -Xmx para no
// pisar la elección del usuario.
func UserHeapOverride() bool {
	for _, name := range []string{EnvJavaMaxHeap, "JAVA_OPTS", "JAVA_TOOL_OPTIONS", "JDK_JAVA_OPTIONS"} {
		if strings.Contains(os.Getenv(name), "-Xmx") {
			return true
		}
	}
	return false
}

// JavaOpts devuelve las opciones extra que el usuario quiere pasar a las JVM.
func JavaOpts() string { return strings.TrimSpace(os.Getenv(EnvJavaOpts)) }

// GoMemoryLimit devuelve el límite blando de memoria para el heap de Go, en
// bytes, o 0 si no hay que aplicar ninguno.
//
// Prioridad: RAIN_GOMEMLIMIT > GOMEMLIMIT (ya la aplica el runtime al
// arrancar) > proot > límite de cgroup detectado.
func GoMemoryLimit() int64 {
	if v := strings.TrimSpace(os.Getenv(EnvGoMemLimit)); v != "" {
		if strings.EqualFold(v, "auto") {
			return autoGoLimit()
		}
		if n, ok := ParseSize(v); ok && n > 0 {
			return n
		}
	}
	if os.Getenv("GOMEMLIMIT") != "" {
		// El runtime de Go ya aplica GOMEMLIMIT al arrancar: no hay que tocarlo.
		return 0
	}
	if UnderProot() {
		return prootLimit()
	}
	return autoGoLimit()
}

// GoGCPercent devuelve el porcentaje de GC que Rain debe aplicar al runtime de
// Go, o 0 si no hay que tocarlo.
//
// Prioridad: RAIN_GOGC > GOGC (ya la aplica el runtime al arrancar) > proot >
// valor derivado de la presión de memoria.
func GoGCPercent() int {
	if v := strings.TrimSpace(os.Getenv(EnvGoGCPercent)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if os.Getenv("GOGC") != "" {
		// El runtime de Go ya aplica GOGC al arrancar.
		return 0
	}
	if UnderProot() {
		return prootGCPercent
	}
	if Detect().Tight() {
		// Con memoria justa, un GC más agresivo mantiene el heap bajo a costa
		// de algo de CPU: mejor que morirse por OOM.
		return tightGCPercent
	}
	return 0
}

// UnderProot indica si el proceso se ejecuta dentro de proot (proot-distro
// en Termux, por ejemplo).
//
// proot reimplementa las llamadas al sistema con ptrace. El runtime de Go
// interactúa con el kernel de una forma que proot no reproduce bien y acaba
// con el heap corrupto:
//
//	fatal error: found pointer to free object
//	SIGSEGV: segmentation violation
//	    runtime.findRunnable
//
// Ninguno de los dos es culpa de Rain: ocurren dentro del runtime, esperando
// a un proceso hijo, y solo bajo proot.
//
// La detección se basa en dos señales:
//   - las variables que exporta proot-distro (PROOT_TMP_DIR y familia),
//   - el proceso aparece trazado (TracerPid != 0), porque proot intercepta
//     las syscalls con ptrace.
func UnderProot() bool {
	return prootDetector()
}

// prootDetector es una variable para que los tests puedan simular el entorno
// sin depender de cómo se esté ejecutando la suite.
var prootDetector = detectProot

// detectProot busca señales de proot en el entorno y en el proceso.
func detectProot() bool {
	for _, name := range []string{"PROOT_TMP_DIR", "PROOT_L2S_DIR", "PROOT_NO_SECCOMP", "PROOT_LOADER"} {
		if os.Getenv(name) != "" {
			return true
		}
	}
	return tracerPID() > 0
}

// tracerPID devuelve el PID del proceso que está trazando el actual, o 0 si
// nadie lo hace.
func tracerPID() int {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "TracerPid:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

// prootLimit es el límite blando que Rain aplica bajo proot: el mismo que
// evita los crashes en la práctica (GOMEMLIMIT=1200MiB). Si el presupuesto de
// memoria es menor, se ajusta a la mitad para no ahogar al proceso.
func prootLimit() int64 {
	limit := int64(prootGoLimit)
	if budget := Detect().Budget(); budget > 0 && budget/2 < limit {
		limit = budget / 2
	}
	if limit < minHeapBytes {
		limit = minHeapBytes
	}
	return limit
}

// autoGoLimit deriva el límite que se aplica al heap de Go.
//
// Sin cgroup no hay un límite duro conocido, así que solo se acota cuando la
// memoria está claramente justa: en ese caso la mitad del presupuesto, con un
// tope de 1 GiB. Un límite blando demasiado agresivo provoca espirales de GC
// que ralentizan más de lo que ahorran, así que en una máquina cómoda no se
// aplica ninguno.
func autoGoLimit() int64 {
	info := Detect()
	if info.CgroupLimitBytes > 0 {
		return info.CgroupLimitBytes * 3 / 4
	}
	if !info.Tight() {
		return 0
	}
	limit := info.Budget() / 2
	if limit > maxAutoGoLimit {
		limit = maxAutoGoLimit
	}
	if limit < minHeapBytes {
		limit = minHeapBytes
	}
	return limit
}

// ApplyGoLimit aplica el límite blando al runtime de Go. Es seguro llamarlo
// aunque no haya límite: en ese caso no hace nada.
func ApplyGoLimit() int64 {
	limit := GoMemoryLimit()
	if limit > 0 {
		debug.SetMemoryLimit(limit)
	}
	return limit
}

// ApplyGoGC aplica el porcentaje de GC al runtime de Go, si Rain debe
// gestionarlo. Devuelve el valor aplicado (0 = sin cambios).
func ApplyGoGC() int {
	percent := GoGCPercent()
	if percent > 0 {
		debug.SetGCPercent(percent)
	}
	return percent
}

// Describe resume la situación de memoria para `rain doctor`.
func Describe() string {
	info := Detect()
	parts := []string{
		fmt.Sprintf("total %s", FormatBytes(info.TotalBytes)),
	}
	if info.HasCgroupLimit() {
		parts = append(parts, fmt.Sprintf("cgroup %s", FormatBytes(info.CgroupLimitBytes)))
	}
	parts = append(parts, fmt.Sprintf("disponible %s", FormatBytes(info.Budget())))
	parts = append(parts, fmt.Sprintf("heap JVM %s", HeapLimit(DefaultKotlincHeap)))

	if limit := GoMemoryLimit(); limit > 0 {
		parts = append(parts, fmt.Sprintf("límite Go %s", FormatBytes(limit)))
	} else if os.Getenv("GOMEMLIMIT") != "" {
		parts = append(parts, "GOMEMLIMIT="+os.Getenv("GOMEMLIMIT"))
	} else {
		parts = append(parts, "GOMEMLIMIT sin definir")
	}

	if percent := GoGCPercent(); percent > 0 {
		parts = append(parts, fmt.Sprintf("GOGC %d (automático)", percent))
	} else if os.Getenv("GOGC") != "" {
		parts = append(parts, "GOGC="+os.Getenv("GOGC"))
	} else {
		parts = append(parts, "GOGC sin definir")
	}

	if UnderProot() {
		parts = append(parts, "proot (GC conservador aplicado)")
	}

	return strings.Join(parts, ", ")
}

// Hint devuelve una recomendación cuando la memoria es escasa, o "" si todo
// está bien.
func Hint() string {
	info := Detect()
	if !info.Tight() && !UnderProot() {
		return ""
	}
	if UnderProot() {
		return "proot detectado: GC conservador aplicado (GOGC=15, GOMEMLIMIT=1200MiB) para que el runtime de Go no se corrompa"
	}
	return fmt.Sprintf("memoria justa: export GOGC=15 GOMEMLIMIT=1200MiB (o %s=1g / %s=1GiB)",
		EnvJavaMaxHeap, EnvGoMemLimit)
}

// ParseSize convierte un tamaño de memoria ("1g", "512m", "2GiB", "1048576")
// en bytes.
func ParseSize(value string) (int64, bool) {
	s := strings.TrimSpace(strings.ToLower(value))
	if s == "" {
		return 0, false
	}

	multiplier := int64(1)
	switch {
	case strings.HasSuffix(s, "gib"):
		multiplier = 1 << 30
		s = strings.TrimSuffix(s, "gib")
	case strings.HasSuffix(s, "mib"):
		multiplier = 1 << 20
		s = strings.TrimSuffix(s, "mib")
	case strings.HasSuffix(s, "kib"):
		multiplier = 1 << 10
		s = strings.TrimSuffix(s, "kib")
	case strings.HasSuffix(s, "gb"):
		multiplier = 1 << 30
		s = strings.TrimSuffix(s, "gb")
	case strings.HasSuffix(s, "mb"):
		multiplier = 1 << 20
		s = strings.TrimSuffix(s, "mb")
	case strings.HasSuffix(s, "kb"):
		multiplier = 1 << 10
		s = strings.TrimSuffix(s, "kb")
	case strings.HasSuffix(s, "g"):
		multiplier = 1 << 30
		s = strings.TrimSuffix(s, "g")
	case strings.HasSuffix(s, "m"):
		multiplier = 1 << 20
		s = strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "k"):
		multiplier = 1 << 10
		s = strings.TrimSuffix(s, "k")
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n * multiplier, true
}

// FormatSize convierte bytes en un valor legible para -Xmx, redondeado hacia
// abajo a múltiplos de 128 MiB.
func FormatSize(bytes int64) string {
	if bytes <= 0 {
		return "0m"
	}
	gran := bytes / heapGranularity * heapGranularity
	if gran < bytes {
		// Redondeo hacia abajo: mejor quedarse corto que pasarse.
		gran = bytes / heapGranularity * heapGranularity
	}
	if gran < minHeapBytes {
		gran = minHeapBytes
	}
	return fmt.Sprintf("%dm", gran/(1<<20))
}

// FormatBytes formatea bytes en unidades legibles (B, KiB, MiB, GiB).
func FormatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(bytes)
	idx := 0
	for value >= 1024 && idx < len(units)-1 {
		value /= 1024
		idx++
	}
	if idx == 0 {
		return fmt.Sprintf("%d B", bytes)
	}
	return fmt.Sprintf("%.1f %s", value, units[idx])
}

func readInt64File(path string) (int64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	text := strings.TrimSpace(string(data))
	if text == "max" {
		return 0, false
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func readMemInfo() (total, available int64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = parseMemInfoValue(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			available = parseMemInfoValue(line)
		}
	}
	return total, available
}

func parseMemInfoValue(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	kb, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return kb * 1024
}
