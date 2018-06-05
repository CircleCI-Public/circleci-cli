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

func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.verbose {
		l.debug.Printf(msg, args...)
	}
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.info.Printf(msg, args...)
}

func (l *Logger) Error(msg string, err error) {
	if err == nil {
		l.error.Println(msg)
	} else {
		l.error.Fatalln(msg, err.Error())
	}
}
