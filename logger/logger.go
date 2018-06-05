package logger

import (
	"log"
	"os"
)

type Logger struct {
	debug   *log.Logger
	info    *log.Logger
	error   *log.Logger
	verbose bool
}

func NewLogger(verbose bool) *Logger {
	return &Logger{
		log.New(os.Stdout, "", 0),
		log.New(os.Stdout, "", 0),
		log.New(os.Stderr, "", 0),
		verbose,
	}
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if l.verbose {
		l.debug.Printf(format, args...)
	}
}

func (l *Logger) Info(args ...interface{}) {
	l.info.Print(args...)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.info.Printf(format, args...)
}

func (l *Logger) Error(msg string, err error) {
	l.error.Print(msg, err.Error())
}

func (l *Logger) FatalOnError(msg string, err error) {
	if err != nil {
		l.error.Fatalln(msg, err.Error())
	}
}
