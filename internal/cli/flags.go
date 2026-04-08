package cli

import (
	"errors"
	"fmt"
	"strings"
)

func parseGlobalFlags(args []string) (string, bool, []string, error) {
	levelName := "info"
	showVersion := false
	i := 0

	for i < len(args) {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			break
		}

		switch {
		case arg == "--log-level":
			if i+1 >= len(args) {
				return "", false, nil, errors.New("missing value for --log-level")
			}
			levelName = args[i+1]
			i += 2
		case strings.HasPrefix(arg, "--log-level="):
			levelName = strings.TrimPrefix(arg, "--log-level=")
			i++
		case arg == "--version" || arg == "-v":
			showVersion = true
			i++
		case arg == "-h" || arg == "--help" || arg == "help":
			return levelName, showVersion, args[i:], nil
		default:
			return "", false, nil, fmt.Errorf("unknown global flag: %s", arg)
		}
	}

	return levelName, showVersion, args[i:], nil
}
