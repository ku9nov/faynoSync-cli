package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

func New(in io.Reader, out io.Writer, buildInfoOverride ...BuildInfo) *App {
	if in == nil || out == nil {
		panic("cli.New: in and out must not be nil")
	}
	buildInfo := BuildInfo{
		Version: "dev",
		Commit:  "none",
		Date:    "unknown",
		Channel: "stable",
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
	if buildInfo.Channel == "" {
		buildInfo.Channel = "stable"
	}
	logger := logrus.New()
	logger.SetOutput(out)
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&prettyFormatter{color: colorEnabled(out)})

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

const (
	cReset  = "\x1b[0m"
	cBold   = "\x1b[1m"
	cDim    = "\x1b[2m"
	cRed    = "\x1b[31m"
	cGreen  = "\x1b[32m"
	cYellow = "\x1b[33m"
	cCyan   = "\x1b[36m"
)

func colorEnabled(out io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// prettyFormatter renders each log entry as a colored level badge and message,
// followed by its fields aligned one per line for readability.
type prettyFormatter struct {
	color bool
}

func (f *prettyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	label, color := levelStyle(entry.Level)

	var b strings.Builder
	b.WriteString(f.paint(color+cBold, label))
	b.WriteByte(' ')
	b.WriteString(f.paint(cBold, entry.Message))
	b.WriteByte('\n')

	keys := make([]string, 0, len(entry.Data))
	for key := range entry.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	width := 0
	for _, key := range keys {
		if len(key) > width {
			width = len(key)
		}
	}

	for _, key := range keys {
		padded := fmt.Sprintf("%-*s", width, key)
		fmt.Fprintf(&b, "    %s  %v\n", f.paint(cDim, padded), entry.Data[key])
	}

	return []byte(b.String()), nil
}

func (f *prettyFormatter) paint(code, text string) string {
	if !f.color {
		return text
	}
	return code + text + cReset
}

func levelStyle(level logrus.Level) (string, string) {
	switch level {
	case logrus.TraceLevel, logrus.DebugLevel:
		return "DEBUG", cDim
	case logrus.WarnLevel:
		return "WARN ", cYellow
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		return "ERROR", cRed
	default:
		return "INFO ", cGreen
	}
}
