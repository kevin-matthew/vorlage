// +build js,wasm

package lmlog

import (
	"syscall/js"
)

func Error(message string) {
	js.Println("Error: "+message)
}
func ErrorF(format string, args ...interface{}) {
	js.Printf("Error: "+format, args...)
}

func Debug(message string) {
	js.Println(message)
}

func DebugF(format string, args ...interface{}) {
	js.Printf(format, args...)
}


func Alert(message string) {
	js.Println(message)
}

func AlertF(format string, args ...interface{}) {
	js.Printf(format, args...)
}

func Emerg(message string) {
	js.Println(message)
}

func EmergF(format string, args ...interface{}) {
	js.Printf(format, args...)
}

func Crit(message string) {
	js.Println(message)
}

func CritF(format string, args ...interface{}) {
	js.Printf(format, args...)
}

func Info(message string) {
	js.Println(message)
}

func InfoF(format string, args ...interface{}) {
	js.Printf(format, args...)
}

func Warning(message string) {
	js.Println(message)
}

func WarningF(format string, args ...interface{}) {
	js.Printf(format, args...)
}

func Notice(message string) {
	js.Println(message)
}

func NoticeF(format string, args ...interface{}) {
	js.Printf(format, args...)
}

func DebugRawF(format string, args ...interface{}) {
	js.Printf(format, args...)
}