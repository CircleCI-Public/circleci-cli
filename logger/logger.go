package logger

import (
	"encoding/json"
	"log"
	"os"
)

// Logger wraps a few log.Logger instances in private fields.
// They are accessible via their respective methods.
type Logger struct {
	debug   *log.Logger
	info    *log.Logger
	error   *log.Logger
	verbose bool
}

// NewLogger returns a reference to a Logger.
// We usually call this when initializing cmd.Logger.
// Later we pass this to client.NewClient so it can also log.
// By default debug and error go to os.Stderr, and info goes to os.Stdout
func NewLogger(verbose bool) *Logger {
	return &Logger{
		log.New(os.Stderr, "", 0),
		log.New(os.Stdout, "", 0),
		log.New(os.Stderr, "", 0),
		verbose,
	}
}

// Debug prints a formatted message to stderr only if verbose is set.
// Consider these messages useful for developers of the CLI.
// This method wraps log.Logger.Printf
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.verbose {
		l.debug.Printf(format, args...)
	}
}

// Info prints all args to os.Stdout
// It's commonly used for messages we want to show the user.
// This method wraps log.Logger.Print
func (l *Logger) Info(args ...interface{}) {
	l.info.Print(args...)
}

// Infoln prints all args to os.Stdout followed by a newline.
// This method wraps log.Logger.Println
func (l *Logger) Infoln(args ...interface{}) {
	l.info.Println(args...)
}

// Infof prints a formatted message to stdout
// This method wraps log.Logger.Printf
func (l *Logger) Infof(format string, args ...interface{}) {
	l.info.Printf(format, args...)
}

// Error prints a message and the given error's message to os.Stderr
// This method wraps log.Logger.Print
func (l *Logger) Error(msg string, err error) {
	if err != nil {
		l.error.Print(msg, err.Error())
	}
}

// FatalOnError prints a message and error's message to os.Stderr then QUITS!
// Please be aware this method will exit the program via os.Exit(1).
// This method wraps log.Logger.Fatalln
func (l *Logger) FatalOnError(msg string, err error) {
	if err != nil {
		l.error.Fatalln(msg, err.Error())
	}
}

// Prettyify accepts a map fo data and pretty prints it.
// It's using json.MarshalIndent and printing with log.Logger.Infoln
func (l *Logger) Prettyify(data map[string]interface{}) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		l.error.Fatalln(err)
	}
	l.Infoln(string(bytes))
}
