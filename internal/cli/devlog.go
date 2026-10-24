package cli

import (
	"fmt"
	"io"
	"os"
)

// DevlogEnabled indica si se deben imprimir los logs de desarrollo.
var DevlogEnabled bool

// DevlogOut es el destino de salida para devlog (por defecto os.Stdout).
var DevlogOut io.Writer = os.Stdout

// devlog imprime por consola el nombre de la función, sus parámetros y
// una frase descriptiva de lo que hace esa función si el flag --devlog está activado.
func devlog(fnName string, params string, description string) {
	if !DevlogEnabled {
		return
	}
	out := DevlogOut
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprintf(out, "[DEVLOG] Función: %s | Parámetros: %s | Descripción: %s\n", fnName, params, description)
}
