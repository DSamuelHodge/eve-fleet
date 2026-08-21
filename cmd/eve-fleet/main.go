package main

import (
	"os"

	"github.com/DSamuelHodge/eve-fleet/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
