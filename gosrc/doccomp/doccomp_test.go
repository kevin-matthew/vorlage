package doccomp

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestLoadDocument(t *testing.T) {
	// change cwd to caller
	//_, filename, _, _ := runtime.Caller(0)
	// The ".." may change depending on you folder structure
	//dir := path.Join(path.Dir(filename), "..")
	err := os.Chdir("../../")

	if err != nil {
		panic(err)
	}

	d, err := LoadDocument("tests/documents/defines-and-includes.dc")
	if err != nil {
		t.Log(err.Error())
		t.Fail()
		return
	}

	res, cerr := ioutil.ReadAll(&d)
	if cerr != nil {
		t.Log(cerr.Error())
		t.Fail()
		return
	}

	print(res)
}
