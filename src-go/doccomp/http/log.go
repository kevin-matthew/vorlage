package http

import (
	"fmt"
	"io"
	"os"
)

const (
	red    = "\033[1;31m"
	yellow = "\033[1;33m"
	white  = "\033[1;37m"
	cyan   = "\033[1;36m"
)

// TODO:  implement the below
// all levels from /usr/include/sys/syslog.h are included here.
type Level uint

const (
	LEVEL_EMERG Level = 1 << (32 - 1 - iota)
	LEVEL_ALERT
	LEVEL_CRIT
	LEVEL_ERR
	LEVEL_WARNING
	LEVEL_NOTICE
	LEVEL_INFO
	LEVEL_DEBUG
)

var LevelMask Level = LEVEL_EMERG |
	LEVEL_ALERT |
	LEVEL_CRIT |
	LEVEL_ERR |
	LEVEL_WARNING |
	LEVEL_NOTICE |
	LEVEL_INFO |
	LEVEL_DEBUG

func IsEnabled(l Level) bool {
	return LevelMask&l != 0
}

func Disable(l Level) {
	LevelMask = LevelMask &^ l
}

// Feel free to set this to whatever you'd like. All output of a http should be going to the same place.
// Thus we don't need to use os.Stderr for example (we should only use that if there was a
// problem starting the http).
var Output io.Writer = os.Stdout

func logColor(prefix string, color string, message string) {
	_, _ = fmt.Fprintf(Output, "[ %s%.6s \033[0m] %s\n", color, prefix, message)
}

func Emerg(message string) {
	if IsEnabled(LEVEL_EMERG) {
		logColor("Emerg", red, message)
	}
}

func Error(message string) {
	if IsEnabled(LEVEL_ERR) {
		logColor("Error", red, message)
	}
}

func Crit(message string) {
	if IsEnabled(LEVEL_CRIT) {
		logColor("Crit", red, message)
	}
}

func Alert(message string) {
	if IsEnabled(LEVEL_ALERT) {
		logColor("Alert", yellow, message)
	}
}

func Warning(message string) {
	if IsEnabled(LEVEL_WARNING) {
		logColor("Warning", yellow, message)
	}
}

func Notice(message string) {
	if IsEnabled(LEVEL_NOTICE) {
		logColor("Notice", yellow, message)
	}
}

func Info(message string) {
	if IsEnabled(LEVEL_INFO) {
		logColor("Alert", white, message)
	}
}

func Debug(message string) {
	if IsEnabled(LEVEL_DEBUG) {
		logColor("Debug", cyan, message)
	}
}
