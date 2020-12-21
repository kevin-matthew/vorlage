package procload

import (
	"plugin"
	"testing"
)

func TestLoadGoProcessors(t *testing.T) {
	p,err := plugin.Open("go.src/golibtestproc.so")
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	sym,err := p.Lookup("Test")
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	rid := sym.(func() Rid)()
	t.Log(rid)
	t.Fail()
}
