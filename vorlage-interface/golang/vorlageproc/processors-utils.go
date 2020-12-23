package vorlageproc

import "io"


type StringBuffer struct {
	String   string
	seeker   int
}
var _ Definition = &StringBuffer{}
func (d *StringBuffer) Close() error {
	d.seeker = 0
	return nil
}
func (d *StringBuffer) Read(p []byte) (int, error) {
	if d.seeker == len(d.String) {
		return 0, io.EOF
	}
	n := copy(p, d.String[d.seeker:])
	if d.seeker+n >= len(d.String) {
		d.seeker = len(d.String)
		return n, io.EOF
	}
	d.seeker += n
	return n, nil
}
func (d *StringBuffer) Reset() error {
	d.seeker = 0
	return nil
}


func (d DefineInfo) Variable() ProcessorVariable {
	return d.RequestInfo.ProcessorInfo.Variables[d.ProcVarIndex]
}
