package main

import (
	"context"
	"os"

	"github.com/gocools-LLC/nard.gocools/internal/cli"
)

var version = "dev"

func main() {
	app := cli.NewApp(version, os.Stdout, os.Stderr)
	code := app.Run(context.Background(), os.Args[1:])
	os.Exit(code)
}
