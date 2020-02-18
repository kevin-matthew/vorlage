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
	cerr := os.Chdir("../../")

	if cerr != nil {
		panic(cerr)
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

	finalFile, cerr := ioutil.ReadFile(
		"tests/documents/final-defines-and-includes.txt")
	if cerr != nil {
		t.Log(cerr.Error())
		t.Fail()
		return
	}

	if string(res) != string(finalFile) {
		t.Log("defines-and-includes.dc does not match final-defines-and-includes.txt")
		t.Log("defines-and-includes:")
		t.Log("'''" + string(res) + "'''")
		t.Log("final-defines-and-includes.txt:")
		t.Log("'''" + string(finalFile) + "'''")
		t.Fail()
	}
}
