package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"faynoSync-cli/internal/config"

	"github.com/sirupsen/logrus"
)

type App struct {
	in     io.Reader
	out    io.Writer
	br     *bufio.Reader
	logger *logrus.Logger
}

func (a *App) Run(args []string) error {
	levelName, remaining, err := parseGlobalFlags(args)
	if err != nil {
		return err
	}
	if err := a.setLogLevel(levelName); err != nil {
		return err
	}

	args = remaining

	if len(args) == 0 {
		a.printRootUsage()
		return nil
	}

	switch args[0] {
	case "init":
		return a.initConfig()
	case "config":
		return a.runConfig(args[1:])
	case "upload":
		return a.runUpload(args[1:])
	case "-h", "--help", "help":
		a.printRootUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func (a *App) runConfig(args []string) error {
	if len(args) == 0 {
		a.printConfigUsage()
		return nil
	}

	switch args[0] {
	case "view":
		return a.viewConfig()
	case "set":
		return a.setConfig(args[1:])
	case "-h", "--help", "help":
		a.printConfigUsage()
		return nil
	default:
		return fmt.Errorf("unknown config command: %s", args[0])
	}
}

func (a *App) initConfig() error {

	path, err := config.Path()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		a.logger.WithField("path", path).Info("Config already exists")
		return nil
	}

	server, err := a.promptValueWithDefault("server", config.DefaultServer)
	if err != nil {
		return err
	}
	a.logger.WithField("server", server).Debug("Server value")
	owner, err := a.promptValueWithDefault("owner", config.DefaultOwner)
	if err != nil {
		return err
	}
	a.logger.WithField("owner", owner).Debug("Owner value")
	path, err = config.Init(config.Config{
		Server: server,
		Owner:  owner,
	})
	if err != nil {
		return err
	}

	a.logger.WithField("path", path).Info("Config initialized")
	return nil
}

func (a *App) viewConfig() error {
	cfg, _, err := config.Load()
	if err != nil {
		return err
	}

	out, err := config.Marshal(cfg)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprint(a.out, string(out))
	return nil
}

func (a *App) setConfig(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: faynosync config set <server|owner> [value]")
	}

	key := args[0]
	value := ""
	if len(args) >= 2 {
		value = args[1]
	} else {
		prompted, err := a.promptValue(key)
		if err != nil {
			return err
		}
		value = prompted
	}

	if value == "" {
		return errors.New("value cannot be empty")
	}

	cfg, path, err := config.Load()
	if err != nil {
		return err
	}

	if err := config.UpdateField(&cfg, key, value); err != nil {
		return err
	}

	if err := config.SaveAt(path, cfg); err != nil {
		return err
	}

	a.logger.WithField("key", key).Info("Config updated")
	return nil
}

func (a *App) promptValue(key string) (string, error) {
	_, _ = fmt.Fprintf(a.out, "Enter value for %s: ", key)

	line, err := a.br.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	return strings.TrimSpace(line), nil
}

func (a *App) promptValueWithDefault(key, defaultValue string) (string, error) {
	_, _ = fmt.Fprintf(a.out, "Enter value for %s [%s]: ", key, defaultValue)

	line, err := a.br.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	value := strings.TrimSpace(line)
	if value == "" {
		return defaultValue, nil
	}

	return value, nil
}

func (a *App) printRootUsage() {
	_, _ = fmt.Fprintln(a.out, `faynosync CLI

Usage:
  faynosync [--log-level <level>] <command>

Global flags:
  --log-level <level>    trace|debug|info|warn|error|fatal|panic (default: info)

Commands:
  faynosync init
  faynosync config view
  faynosync config set <server|owner> [value]
  faynosync upload [flags]`)
}

func (a *App) printConfigUsage() {
	_, _ = fmt.Fprintln(a.out, `faynosync config commands

Usage:
  faynosync config view
  faynosync config set <server|owner> [value]`)
}
