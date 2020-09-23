package simple

import (
	".."
	"io"
	"strings"
)

type lowVolumeProcessor struct {
	Description string
	Variables   map[string]CallbackDefinition
}

var _ doccomp.Processor = lowVolumeProcessor{}

type CallbackDefinition struct {
	Description    string
	DefineFunc     func(args map[string]string) string
	RequiredFields []string
}

type stringDef struct {
	reader *strings.Reader
}

func (s stringDef) Reset() error {
	_, err := s.reader.Seek(0, 0)
	return err
}

func (s stringDef) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

func newStringDef(s string) stringDef {
	return stringDef{strings.NewReader(s)}
}

func (l lowVolumeProcessor) GetDescription() string {
	return l.Description
}

func (l lowVolumeProcessor) GetVariables() []doccomp.ProcessorVariable {
	var ret []doccomp.ProcessorVariable
	for k, v := range l.Variables {
		r := doccomp.ProcessorVariable{
			Name:               k,
			Description:        v.Description,
			InputNames:         v.RequiredFields,
			StreamedInputNames: nil,
		}
		ret = append(ret, r)
	}
	return ret
}

func (l lowVolumeProcessor) DefineVariable(name string, input map[string]string, streams map[string]io.Reader) (def doccomp.Definition, err error) {
	ret := l.Variables[name].DefineFunc(input)

	return newStringDef(ret), nil
}

/*
 * Make a simple processor met for low-volume traffic and low-volume i/o.
 * You should take what is returned here and add it to doccomp.Processors.
 *
 * Note that this processor is met for low-volume traffic and each definition
 * will have to be treated independantly.
 */
func NewProcessor(Description string, Variables map[string]CallbackDefinition) doccomp.Processor {
	return lowVolumeProcessor{Description: Description, Variables: Variables}
}
