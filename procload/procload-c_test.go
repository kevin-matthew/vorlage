package procload

import (
	"fmt"
	"io"
	"io/ioutil"
	"testing"
)
type ezstream struct {
	int
	string
}

func (e *ezstream) Read(p []byte) (int,error ){
	var i int
	if e.int == len(e.string) {
		return 0, io.EOF
	}
	for i = 0; i < len(p) && i+e.int < len(e.string); i++ {
		p[i] = e.string[e.int+i]
	}
	e.int += i
	return i, nil
}

func (e *ezstream) Close() error {
	fmt.Printf("closing!\n")
	return nil
}

func TestLoadc(t *testing.T) {
	p, err := dlOpen("./c.src/libtest.so")
	if err != nil {
		t.Logf("failed to open dl: %s\n", err)
		t.Fail()
		return
	}
	err = p.loadVorlageSymbols()
	if err != nil {
		t.Logf("failed to load syms: %s\n", err)
		t.Fail()
		return
	}
	info := p.Startup()
	t.Logf("processor info: %#v\n", info)

	r := RequestInfo{
		ProcessorInfo: &info,
		Filepath:      "./c.src/test.txt",
		Input: []string{
			"hey this is a test dude log this",
		},
		StreamInput: []StreamInput{
			&ezstream{0, "pussy ass bitch"},
		},
	}

	var context interface{}
	actions := p.OnRequest(r, &context)
	for _, a := range actions {
		t.Logf("action id: %0.8x\n", a.Action)
		t.Logf("action data: %#v\n", string(a.Data))
	}


	def := DefineInfo{
		RequestInfo:       &r,
		ProcVarIndex: 0,
		Input: []string{
				"echo me god damnit!",
		},
		StreamInput: nil,
	}
	varible := p.DefineVariable(def, context)

	//buffer := make([]byte, 1000);
	buffer,err := ioutil.ReadAll(varible)
		if err != nil {
		t.Logf("error when reading def: %s\n", err.Error())
		t.Fail()
		return
	}
	t.Logf("variable: %s\n", string(buffer))

	err = varible.Close()
	if err != nil {
		t.Logf("failed to close variable: %s\n", err)
		t.Fail()
		return
	}

	p.OnFinish(r, context)

	err = p.Shutdown()
	if err != nil {
		t.Logf("failed to close processor: %s\n", err)
		t.Fail()
		return
	}

	t.Fail()
}
