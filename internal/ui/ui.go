package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	Gray   = "\033[90m"
)

var (
	Verbose bool
	Quiet   bool
	NoColor bool
)

// Out y Err permiten redirigir la salida (tests, embebido). Por defecto van a
// os.Stdout/os.Stderr.
var (
	Out io.Writer = os.Stdout
	Err io.Writer = os.Stderr
)

// SetOutput redirige todos los mensajes de ui a los writers dados.
func SetOutput(out, errOut io.Writer) {
	if out != nil {
		Out = out
	}
	if errOut != nil {
		Err = errOut
	}
}

func paint(code, text string) string {
	if NoColor {
		return text
	}
	return code + text + Reset
}

func Success(format string, a ...interface{}) {
	if Quiet {
		return
	}
	fmt.Fprintf(Out, "%s "+format+"\n", append([]interface{}{paint(Green, "✅ ")}, a...)...)
}

func Warn(format string, a ...interface{}) {
	if Quiet {
		return
	}
	fmt.Fprintf(Out, "%s "+format+"\n", append([]interface{}{paint(Yellow, "⚠️  ")}, a...)...)
}

func Error(format string, a ...interface{}) {
	fmt.Fprintf(Err, "%s "+format+"\n", append([]interface{}{paint(Red, "❌ ")}, a...)...)
}

func Info(format string, a ...interface{}) {
	if Quiet {
		return
	}
	fmt.Fprintf(Out, "%s "+format+"\n", append([]interface{}{paint(Cyan, "[rain] ")}, a...)...)
}

func VerboseLog(format string, a ...interface{}) {
	if !Verbose || Quiet {
		return
	}
	fmt.Fprintf(Out, "%s "+format+"\n", append([]interface{}{paint(Gray, "       ")}, a...)...)
}

// ProgressBar dibuja una barra de progreso de una sola línea.
type ProgressBar struct {
	Total   int
	Current int
	Width   int
	Label   string
}

func NewProgressBar(total int, label string) *ProgressBar {
	return &ProgressBar{
		Total: total,
		Width: 20,
		Label: label,
	}
}

// Render dibuja la barra en la línea actual (sin salto de línea).
func (p *ProgressBar) Render() {
	if p.Total <= 0 || Quiet {
		return
	}
	if p.Current > p.Total {
		p.Current = p.Total
	}
	percentage := float64(p.Current) / float64(p.Total)
	filled := int(percentage * float64(p.Width))
	empty := p.Width - filled

	color := Cyan
	if percentage > 0.33 {
		color = Blue
	}
	if percentage > 0.66 {
		color = Green
	}
	if NoColor {
		color = ""
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	if NoColor {
		fmt.Fprintf(Out, "\r[%s] %3d%% %s", bar, int(percentage*100), strings.Repeat(" ", 5))
		return
	}
	fmt.Fprintf(Out, "\033[2K\r%s[%s%s%s] %3d%% %s",
		p.Label, color, bar, Reset, int(percentage*100), strings.Repeat(" ", 5))
	if f, ok := Out.(*os.File); ok {
		_ = f.Sync()
	}
}

func (p *ProgressBar) Finish() {
	if Quiet {
		return
	}
	p.Current = p.Total
	p.Render()
	fmt.Fprintln(Out)
}
