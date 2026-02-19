package main

import (
	"fmt"
	"os"

	"faynoSync-cli/internal/cli"
)

func main() {
	app := cli.New(os.Stdin, os.Stdout)
	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
