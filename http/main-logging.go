package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type logcontext struct {
	context string
	c *logChannels
}

type logChannels struct {
	Debug string
	debug *os.File
	Verbose string
	verbose *os.File
	Warnings string
	warnings *os.File
	Errors string
	errors *os.File
	Failures string
	failures *os.File
}

func (l *logChannels) LoadChannels() (err error) {

	// close the old one if it was open.
	if l.debug != nil {
		_ = l.debug.Close()
		l.debug = nil
	}
	if l.Debug != "" {
		l.debug,err = os.OpenFile(l.Debug, os.O_APPEND| os.O_WRONLY, os.ModePerm)
		if err != nil {
			return err
		}
	}

	// close the old one if it was open.
	if l.verbose != nil {
		_ = l.verbose.Close()
		l.verbose = nil
	}
	if l.Verbose != "" {
		l.verbose,err = os.OpenFile(l.Verbose, os.O_APPEND | os.O_WRONLY, os.ModePerm)
		if err != nil {
			return err
		}
	}

	// close the old one if it was open.
	if l.warnings != nil {
		_ = l.warnings.Close()
		l.warnings = nil
	}
	if l.Warnings != "" {
		l.warnings,err = os.OpenFile(l.Warnings, os.O_APPEND| os.O_WRONLY, os.ModePerm)
		if err != nil {
			return err
		}
	}

	// close the old one if it was open.
	if l.errors != nil {
		_ = l.errors.Close()
		l.errors = nil
	}
	if l.Errors != "" {
		l.errors,err = os.OpenFile(l.Errors, os.O_APPEND| os.O_WRONLY, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

var logs = logChannels{
	Debug: "",
	Verbose: "/dev/stdout",
	Warnings: "/dev/stdout",
	Errors: "/dev/stderr",
	Failures: "/dev/stderr",
}

func (l logcontext) Emergf(format string, args ...interface{}) {
	logToFile(l.c.failures, "alert", 0, l.context, format, args...)
}

func (l logcontext) Critf(format string, args ...interface{}) {
	logToFile(l.c.failures, "alert", 0, l.context, format, args...)
}

func (l logcontext) Alertf(format string, args ...interface{}) {
	logToFile(l.c.failures, "alert", 0, l.context, format, args...)
}

func (l logcontext) Warnf(format string, args ...interface{}) {
	logToFile(l.c.warnings, "warnings", 0, l.context, format, args...)
}

func (l logcontext) Noticef(format string, args ...interface{}) {
	logToFile(l.c.verbose, "notice", 0, l.context, format, args...)
}

func (l logcontext) Errorf(format string, args ...interface{}) {
	logToFile(l.c.errors, "error", 0, l.context, format, args...)
}

func (l logcontext) Infof(format string, args ...interface{}) {
	logToFile(l.c.verbose, "info", 0, l.context, format, args...)
}

func (l logcontext) Debugf(format string, args ...interface{}) {
	logToFile(l.c.debug, "debug", 0, l.context, format, args...)
}

func logToFile(file *os.File, channel string, printstack int, context string, format string, args ...interface{}) {
	if file == nil{
		return
	}
	message := fmt.Sprintf(format, args...)
	message = strings.ReplaceAll(message, "\n", "\n["+context + " " + channel+" (cont.)]")
	_,_ = fmt.Fprintf(os.Stdout, "[%s %s %s] %s\n", context, channel, time.Now().Format("2006-01-02T15:04:05"), message)
}