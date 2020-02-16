package doccomp

import "strings"

const DefineMacro = "#define"
const IncludeMacro = "#include"

type DocumentStream struct {}

type Document struct {}

func LoadRequestedDocument(request Request) (Document, *Error) {
	return Document{},errNotImplemented
}

func (d *Document) addDefinition(definitions Definition) {

}

func (d *Document) remainingDefinitions() []Definition {
	return nil
}

func (d *Document) complete() (stream DocumentStream, err *Error) {
	remaining := d.remainingDefinitions()
	if len(remaining) != 0 {
		err := NewError("variables were left undefined")
		// build a nice little string of remaining definitinos
		names := make([]string, len(remaining))
		for i,d := range remaining {
			names[i] = d.GetName()
		}
		subject := strings.Join(names, ", ")
		err.SetSubject(subject)
		return stream,err
	}
	return stream, errNotImplemented
}