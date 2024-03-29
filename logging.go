// note this package is just a shell package. implementations should
// replace Logger with something other than nullLog to format log output.

package vorlage

type Loggert interface {
	Errorf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Noticef(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Alertf(format string, args ...interface{})
}

var Logger Loggert = nullLog{}

type nullLog struct{}

func (n nullLog) Noticef(format string, args ...interface{}) {
}
func (n nullLog) Warnf(format string, args ...interface{}) {
}
func (n nullLog) Errorf(format string, args ...interface{}) {
}
func (n nullLog) Infof(format string, args ...interface{}) {
}
func (n nullLog) Debugf(format string, args ...interface{}) {
}
func (n nullLog) Alertf(format string, args ...interface{}) {
}
