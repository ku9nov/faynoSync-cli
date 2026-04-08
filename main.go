package main

import (
	"fmt"
	"os"

	"faynoSync-cli/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	channel = "stable"
)

func main() {
	app := cli.New(os.Stdin, os.Stdout, cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
		Channel: channel,
	})
	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
