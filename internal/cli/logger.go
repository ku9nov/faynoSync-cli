package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
)

func New(in io.Reader, out io.Writer, buildInfoOverride ...BuildInfo) *App {
	if in == nil || out == nil {
		panic("cli.New: in and out must not be nil")
	}
	buildInfo := BuildInfo{
		Version: "dev",
		Commit:  "none",
		Date:    "unknown",
	}
	if len(buildInfoOverride) > 0 {
		buildInfo = buildInfoOverride[0]
	}
	if buildInfo.Version == "" {
		buildInfo.Version = "dev"
	}
	if buildInfo.Commit == "" {
		buildInfo.Commit = "none"
	}
	if buildInfo.Date == "" {
		buildInfo.Date = "unknown"
	}
	logger := logrus.New()
	logger.SetOutput(out)
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	return &App{
		in:        in,
		out:       out,
		br:        bufio.NewReader(in),
		logger:    logger,
		buildInfo: buildInfo,
	}
}

func (a *App) setLogLevel(levelName string) error {
	level, err := logrus.ParseLevel(strings.ToLower(levelName))
	if err != nil {
		return fmt.Errorf("invalid log level %q", levelName)
	}

	a.logger.SetLevel(level)
	return nil
}
