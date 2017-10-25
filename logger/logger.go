package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/cenkalti/log"
)

var stderrHandler = log.NewFileHandler(os.Stderr)

func init() {
	stderrHandler.SetFormatter(logFormatter{})
	stderrHandler.SetLevel(log.DEBUG)
}

// Logger is used for printing messages in color with various levels.
// Messages are printed to stderr.
// Default message level is INFO.
type Logger struct {
	log.Logger
}

// New creates a new Logger.
func New(name string) *Logger {
	logger := log.NewLogger(name)
	logger.SetHandler(stderrHandler)
	return &Logger{logger}
}

// EnableDebug changes this logger's level to DEBUG. By default it is INFO.
func (l *Logger) EnableDebug() {
	l.SetLevel(log.DEBUG)
	l.Notice("Enabled debug log.")
}

// RecoverAndLog recovers from panic and prints stacktrace as a CRITICAL log message.
// Normally used with a defer statement.
func (l *Logger) RecoverAndLog() {
	if err := recover(); err != nil {
		buf := make([]byte, 10000)
		l.Critical(err, "\n", string(buf[:runtime.Stack(buf, false)]))
	}
}

type logFormatter struct{}

// Format outputs a message like "2014-02-28 18:15:57 [example] INFO     somethinfig happened"
func (f logFormatter) Format(rec *log.Record) string {
	return fmt.Sprintf("%s %-8s [%s] %-8s %s",
		fmt.Sprint(rec.Time)[:19],
		log.LevelNames[rec.Level],
		rec.LoggerName,
		filepath.Base(rec.Filename)+":"+strconv.Itoa(rec.Line),
		rec.Message)
}
