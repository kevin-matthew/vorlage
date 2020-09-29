package simple

import (
	".."
	"io"
	"strings"
)

/*
 * lowVolumeProcessor isn't smart, doesn't cache anything. And does not
 * utilize good caching procedures in leu of easy to work with.
 */
type lowVolumeProcessor struct {
	Description string
	Variables   map[string]CallbackDefinition
}

var _ doccomp.Processor = lowVolumeProcessor{}

/*
 * CallbackDefinition is a doccomp.Processor that has been simplified into
 * terms of "this variable will invoke this function. And that function will
 * return this string".
 */
type CallbackDefinition struct {
	Description    string
	DefineFunc     func(args map[string]string) string
	RequiredFields []string
}

/*
 * make a string reader thats complient with doccomp.Definition
 */
type stringDef struct {
	reader *strings.Reader
}

/*
 * doccomp.Definition
 */
func (s stringDef) Reset() error {
	_, err := s.reader.Seek(0, 0)
	return err
}

/*
 * doccomp.Definition
 */
func (s stringDef) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

/*
 * make a string reader thats complient with doccomp.Definition
 */
func newStringDef(s string) stringDef {
	return stringDef{strings.NewReader(s)}
}

/*
 * implemented doccomp.Processor
 */
func (l lowVolumeProcessor) GetDescription() string {
	return l.Description
}

/*
 * implemented doccomp.Processor
 */
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

/*
 * implemented doccomp.Processor
 */
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
