// +build !js,!wasm

package lmlog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

var ShowFuncLogs bool = true

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

var LevelMask = LEVEL_EMERG |
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
var ErrOurput io.Writer = os.Stderr

var lastP string

func logColor(prefix string, color string, message string, isError bool) {
	//if ShowFuncLogs {
	fullprefix  := fmt.Sprintf("[ %s%-7s\033[0m] %s%s\033[0m: ", color, prefix, white, calleeLocation())
	fullmessage := fullprefix + message + "\n"
	if isError {
		fmt.Fprint(ErrOurput, fullmessage)
	} else {
		fmt.Fprint(Output, fullmessage)
	}
	//	if lastP != P {
	//		_, _ = fmt.Fprintf(Output, "%s\n  » %s\n", P, message)
	//		lastP = P
	//	} else {
	//		_, _ = fmt.Fprintf(Output, "  » %s\n", message)
	//	}
	//} else {
	//	_, _ = fmt.Fprintf(Output, "[ %s%.6s \033[0m] %s\n", color, prefix, message)
	//}
}

/*
 * Emerg, EmergF
 * didn't go as planned, already causing damage or about to
 */
func Emerg(message string) {
	if IsEnabled(LEVEL_EMERG) {
		logColor("Emerg", red, message, true )
	}
}
func EmergF(message string, args ...interface{}) {
	if IsEnabled(LEVEL_EMERG) {
		logColor("Emerg", red, fmt.Sprintf(message, args...), true)
	}
}

/*
 * Error, ErrorF
 * didn't go as planned
 */
func Error(message string) {
	if IsEnabled(LEVEL_ERR) {
		logColor("Error", red, message, true)
	}
}
func ErrorF(message string, args ...interface{}) {
	if IsEnabled(LEVEL_ERR) {
		logColor("Error", red, fmt.Sprintf(message, args...), true)
	}
}

/*
 * Crit, CritF
 * didn't go as planned, has the potential to cause further errors if not
 * addressed
 */
func Crit(message string) {
	if IsEnabled(LEVEL_CRIT) {
		logColor("Crit", red, message, true)
	}
}
func CritF(message string, args ...interface{}) {
	if IsEnabled(LEVEL_CRIT) {
		logColor("Crit", red, fmt.Sprintf(message, args...), true)
	}
}

/*
 * Alert, AlertF
 * unusual event, but expected
 */
func Alert(message string) {
	if IsEnabled(LEVEL_ALERT) {
		logColor("Alert", yellow, message, false)
	}
}
func AlertF(message string, args ...interface{}) {
	if IsEnabled(LEVEL_ALERT) {
		logColor("Alert", yellow, fmt.Sprintf(message, args...), false)
	}
}

/*
 * Warning, WarningF
 * outside of threshold, but not erroneous
 */
func Warning(message string) {
	if IsEnabled(LEVEL_WARNING) {
		logColor("Warning", yellow, message, false)
	}
}
func WarningF(format string, args ...interface{}) {
	if IsEnabled(LEVEL_WARNING) {
		logColor("Warning", yellow, fmt.Sprintf(format, args...), false)
	}
}

/*
 * Notice, NoticeF
 * operating as normal
 */
func Notice(message string) {
	if IsEnabled(LEVEL_NOTICE) {
		logColor("Notice", yellow, message, false)
	}
}
func NoticeF(format string, args ...interface{}) {
	if IsEnabled(LEVEL_NOTICE) {
		logColor("Notice", yellow, fmt.Sprintf(format, args...), false)
	}
}

/*
 * Info, InfoF
 * operating as normal, successful
 */
func Info(message string) {
	if IsEnabled(LEVEL_INFO) {
		logColor("Info", white, message, false)
	}
}
func InfoF(format string, args ...interface{}) {
	if IsEnabled(LEVEL_INFO) {
		logColor("Info", white, fmt.Sprintf(format, args...), false)
	}
}

/*
 * Debug, DebugF
 * Messages for the developer to track progress
 * DebugRawF will not add prefix/suffix
 */
func Debug(message string) {
	if IsEnabled(LEVEL_DEBUG) {
		logColor("Debug", cyan, message, false)
	}
}
func DebugF(format string, args ...interface{}) {
	if IsEnabled(LEVEL_DEBUG) {
		logColor("Debug", cyan, fmt.Sprintf(format, args...), false)
	}
}

func DebugRawF(format string, args ...interface{}) {
	if IsEnabled(LEVEL_DEBUG) {
		_, _ = fmt.Fprintf(Output, format, args...)
	}
}

func calleeLocation() string {
	// We need the frame at index skipFrames+2, since we never want runtime.Callers and getFrame
	targetFrameIndex := 3 + 1

	// Set size to targetFrameIndex+2 to ensure we have room for one more caller than we need
	programCounters := make([]uintptr, targetFrameIndex+2)
	n := runtime.Callers(0, programCounters)

	frame := runtime.Frame{Function: "unknown"}
	if n > 0 {
		frames := runtime.CallersFrames(programCounters[:n])
		for more, frameIndex := true, 0; more && frameIndex <= targetFrameIndex; frameIndex++ {
			var frameCandidate runtime.Frame
			frameCandidate, more = frames.Next()
			if frameIndex == targetFrameIndex {
				frame = frameCandidate
			}
		}
	}
	dir,_ := os.Getwd()
	file,_ := filepath.Rel(dir + "/go-src", frame.File)
	//funcf := frame.Function[strings.LastIndex(frame.Function, "/")+1:]

	return fmt.Sprintf("\033[1m%s:%d\033[0m", file,frame.Line)
}
