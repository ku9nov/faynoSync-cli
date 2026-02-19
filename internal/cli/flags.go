package cli

import (
	"errors"
	"fmt"
	"strings"
)

func parseGlobalFlags(args []string) (string, []string, error) {
	levelName := "info"
	i := 0

	for i < len(args) {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			break
		}

		switch {
		case arg == "--log-level":
			if i+1 >= len(args) {
				return "", nil, errors.New("missing value for --log-level")
			}
			levelName = args[i+1]
			i += 2
		case strings.HasPrefix(arg, "--log-level="):
			levelName = strings.TrimPrefix(arg, "--log-level=")
			i++
		case arg == "-h" || arg == "--help" || arg == "help":
			return levelName, args[i:], nil
		default:
			return "", nil, fmt.Errorf("unknown global flag: %s", arg)
		}
	}

	return levelName, args[i:], nil
}
