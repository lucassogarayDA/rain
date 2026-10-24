package main

import (
	"github.com/itskreisler/rain/internal/cli"
	"github.com/itskreisler/rain/internal/memenv"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// El runtime de Go puede morir por presión de memoria en contenedores
	// pequeños (el famoso "found pointer to free object"). Si el usuario no ha
	// fijado GOMEMLIMIT/GOGC, Rain aplica un límite blando y un GC más
	// agresivo según la memoria disponible.
	memenv.ApplyGoLimit()
	memenv.ApplyGoGC()

	cli.Execute(cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	})
}
