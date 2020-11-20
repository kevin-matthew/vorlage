package vorlage

import (
	"io/ioutil"
	"testing"
)

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
		StreamInput: nil,
		Rid:         0,
	}

	actions := p.OnRequest(r)
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
	varible := p.DefineVariable(def)

	bytes, err := ioutil.ReadAll(varible)
	if err != nil {
		t.Logf("error when reading def: %s\n", err.Error())
		t.Fail()
		return
	}
	t.Logf("variable: %s\n", string(bytes))

	err = p.Close()
	if err != nil {
		t.Logf("failed to close: %s\n", err)
		t.Fail()
		return
	}
}
