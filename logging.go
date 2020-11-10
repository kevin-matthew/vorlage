package vorlag

func SetLogger(l Logger) {
	logger = l
}

type Logger interface {
	Errorf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
}

var logger Logger = nullLog{}

type nullLog struct{}

func (n nullLog) Errorf(format string, args ...interface{}) {
}
func (n nullLog) Infof(format string, args ...interface{}) {
}
func (n nullLog) Debugf(format string, args ...interface{}) {
}
