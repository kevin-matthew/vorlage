package simple

import (
	".."
	"strings"
)

/*
 * lowVolumeProcessor isn't smart, doesn't cache anything. And does not
 * utilize good caching procedures in leu of easy to work with.
 */
type lowVolumeProcessor struct {
	Description string
	Variables   map[string]CallbackDefinition
	Name        string
	variables   []*doccomp.ProcessorVariable
}

func (l lowVolumeProcessor) Info() doccomp.ProcessorInfo {
	l.GetVariables()
	return doccomp.ProcessorInfo{
		Name:        l.Name,
		Description: l.Description,
		Variables:   l.variables,
	}
}

func (l lowVolumeProcessor) PreProcess(rid doccomp.Rid) {
	return
}

func (l lowVolumeProcessor) PostProcess(rid doccomp.Rid) {
	return
}

var _ doccomp.Processor = lowVolumeProcessor{}

/*
 * CallbackDefinition is a doccomp.Processor that has been simplified into
 * terms of "this variable will invoke this function. And that function will
 * return this string".
 */
type CallbackDefinition struct {
	Description    string
	DefineFunc     func(Rid doccomp.Rid, args doccomp.Input) string
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

// helper to info
func (l lowVolumeProcessor) GetVariables() {
	var ret []*doccomp.ProcessorVariable
	for k, v := range l.Variables {
		inputM := make(map[string]string, len(v.RequiredFields))
		for i := range v.RequiredFields {
			inputM[v.RequiredFields[i]] = ""
		}
		r := doccomp.ProcessorVariable{
			Name:          k,
			Description:   v.Description,
			Input:         inputM,
			StreamedInput: nil,
		}
		ret = append(ret, &r)
	}
	l.variables = ret
}

/*
 * implemented doccomp.Processor
 */
func (l lowVolumeProcessor) DefineVariable(rid doccomp.Rid, variable *doccomp.ProcessorVariable) doccomp.Definition {
	ret := l.Variables[variable.Name].DefineFunc(rid, variable.Input)
	return newStringDef(ret)
}

/*
 * Make a simple processor met for low-volume traffic and low-volume i/o.
 * You should take what is returned here and add it to doccomp.Processors.
 *
 * Note that this processor is met for low-volume traffic and each definition
 * will have to be treated independantly.
 */
func NewProcessor(name string, Description string, Variables map[string]CallbackDefinition) doccomp.Processor {
	return lowVolumeProcessor{Name: name, Description: Description, Variables: Variables}
}
