package doccomp

import (
	"io"
	"os"
	"testing"
)
import "../lmlog"

func asdf(a []byte) {
	lmlog.DebugF("test2: %d", len(a))
	a = a[2:]
	lmlog.DebugF("test2: %d", len(a))
	a[0] = 'd'
	return
}

func TestLoadDocument(t *testing.T) {

	// change cwd to caller
	//_, filename, _, _ := runtime.Caller(0)
	// The ".." may change depending on you folder structure
	//dir := path.Join(path.Dir(filename), "..")
	cerr := os.Chdir("../../")

	if cerr != nil {
		panic(cerr)
	}

	d, err := LoadDocument("tests/documents/defines.haml", nil, nil)
	if err != nil {
		t.Log(err.ErrorHighlight())
		t.Fail()
		return
	}

	buff := make([]byte, 10)
	var total = ""
	var totalreads int
	var averageBuffUsage float64
	for {
		totalreads++
		lmlog.DebugF("reading...")
		n, cerr := d.Read(buff)
		if cerr != nil && cerr != io.EOF {
			t.Log(cerr.Error())
			t.Fail()
			return
		}

		averageBuffUsage += float64(n) / float64(len(buff))

		total += string(buff[:n])
		lmlog.Debug("total: " + total)

		if cerr == io.EOF {
			break
		}
	}

	t.Logf("total reads: %d", totalreads)
	t.Logf("average buffer utilization: %.2f%%", (averageBuffUsage/float64(totalreads))*100)

	//println(buff[:n])
	t.Log(total)
	t.Fail()

	return
	/*finalFile, cerr := ioutil.ReadFile(
		"tests/documents/final-defines-and-prepends.txt")
	if cerr != nil {
		t.Log(cerr.Error())
		t.Fail()
		return
	}

	if string(res) != string(finalFile) {
		t.Log("defines-and-prepends.dc does not match final-defines-and-prepends.txt")
		t.Log("defines-and-prepends:")
		t.Log("'''" + string(res) + "'''")
		t.Log("final-defines-and-prepends.txt:")
		t.Log("'''" + string(finalFile) + "'''")
		t.Fail()
		return
	}
	cerr = d.Close()
	if cerr != nil {
		t.Log("failed to close document: " + cerr.Error())
		t.Fail()
		return
	}

	// now we break it

	// circular dep error
	d, err = LoadDocument("tests/documents/include-self.dc")
	_ = d.Close()
	if err == nil {
		t.Log("include-self.dc did not raise a cirular dependcie error")
		t.Fail()
		return
	}
	t.Log("when testing circular dependices (1) got: " + err.ErrorHighlight())
	d, err = LoadDocument("tests/documents/include-cycle-1.dc")
	_ = d.Close()
	if err == nil {
		t.Log("tests/documents/include-cycle-1." +
			"dc did not raise a cirular dependcie error")
		t.Fail()
		return
	}
	t.Log("when testing circular dependices (2) got: " + err.ErrorHighlight())*/
}
