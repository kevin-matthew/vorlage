package vorlage

import (
	"ellem.so/vorlageproc"
	"io/ioutil"
	"os"
	"plugin"
)

type goProc struct {
	sourcefile            string
	plugin                *plugin.Plugin
	vorlageStartup        func() (vorlageproc.ProcessorInfo, error)
	vorlageOnRequest      func(info vorlageproc.RequestInfo, i *interface{}) []vorlageproc.Action
	vorlageDefineVariable func(info vorlageproc.DefineInfo, i interface{}) vorlageproc.Definition
	vorlageOnFinish       func(info vorlageproc.RequestInfo, i interface{})
	vorlageShutdown       func() error

	// set in NewCompiler
	indexincompiler int
}

func goProchandleerr(err error, ok bool, s string) error {
	if err != nil {
		return lmerrorNew(0x153b42,
			"symbol not found",
			err,
			"ensure the processor was properly built and is up to date",
			s)
	}
	if !ok {
		return lmerrorNew(0x153b41,
			"symbol not valid",
			nil,
			"ensure the processor was properly built and is up to date",
			s)
	}
	return nil
}

// used for debugging
func GoAddToPreload(f func() []vorlageproc.VorlageGo) {
	// v2 symbol is valid. Make the call.
	v2procs := f()
	gv := make([]*goProc, len(v2procs))
	for i := range v2procs {
		gv[i] = new(goProc)
		gv[i].sourcefile = "__INTERNAL__"
		gv[i].plugin = nil
		gv[i].vorlageStartup = v2procs[i].VorlageStartup
		gv[i].vorlageOnRequest = v2procs[i].VorlageOnRequest
		gv[i].vorlageDefineVariable = v2procs[i].VorlageDefineVariable
		gv[i].vorlageOnFinish = v2procs[i].VorlageOnFinish
		gv[i].vorlageShutdown = v2procs[i].VorlageShutdown
	}
	// v2 symbol linked successfully.
	preLoadGo = append(preLoadGo, gv...)
}

var preLoadGo []*goProc

// parses a file and returns 1 or many goProc to be used as a vorlageproc.Processor(s).
// does NOT run .VorlageStartup().
func loadGoProc(path string, fname string) (gv []*goProc, err error) {
	plug, err := plugin.Open(path)
	if err != nil {
		return gv, lmerrorNew(3185,
			"failed to open plugin file",
			err,
			"make sure the file is valid",
			path)
	}
	var ok bool
	var sym plugin.Symbol
	var v2procs []vorlageproc.VorlageGo

	// Lets look for the V2 interface...
	var vorlagegov func() []vorlageproc.VorlageGo
	sym, err = plug.Lookup("VorlageGoV")
	if err != nil {
		// symbol not found. Go to the v1 interface
		goto v1
	}
	vorlagegov, ok = sym.(func() []vorlageproc.VorlageGo)
	if e := goProchandleerr(err, ok, "VorlageGoV"); e != nil {
		return gv, e
	}

	// v2 symbol is valid. Make the call.
	v2procs = vorlagegov()
	gv = make([]*goProc, len(v2procs))
	for i := range v2procs {
		gv[i] = new(goProc)
		gv[i].sourcefile = fname
		gv[i].plugin = plug
		gv[i].vorlageStartup = v2procs[i].VorlageStartup
		gv[i].vorlageOnRequest = v2procs[i].VorlageOnRequest
		gv[i].vorlageDefineVariable = v2procs[i].VorlageDefineVariable
		gv[i].vorlageOnFinish = v2procs[i].VorlageOnFinish
		gv[i].vorlageShutdown = v2procs[i].VorlageShutdown
	}
	// v2 symbol linked successfully.
	return gv, nil

v1:
	g := goProc{}
	g.plugin = plug
	sym, err = g.plugin.Lookup("VorlageStartup")
	if err == nil {
		g.vorlageStartup, ok = sym.(func() (vorlageproc.ProcessorInfo, error))
	}
	if e := goProchandleerr(err, ok, "VorlageStartup"); e != nil {
		return gv, e
	}
	sym, err = g.plugin.Lookup("VorlageOnRequest")
	if err == nil {
		g.vorlageOnRequest, ok = sym.(func(info vorlageproc.RequestInfo, i *interface{}) []vorlageproc.Action)
	}
	if e := goProchandleerr(err, ok, "VorlageOnRequest"); e != nil {
		return gv, e
	}
	sym, err = g.plugin.Lookup("VorlageDefineVariable")
	if err == nil {
		g.vorlageDefineVariable, ok = sym.(func(info vorlageproc.DefineInfo, i interface{}) vorlageproc.Definition)
	}
	if e := goProchandleerr(err, ok, "VorlageDefineVariable"); e != nil {
		return gv, e
	}
	sym, err = g.plugin.Lookup("VorlageOnFinish")
	if err == nil {
		g.vorlageOnFinish, ok = sym.(func(info vorlageproc.RequestInfo, i interface{}))
	}
	if e := goProchandleerr(err, ok, "VorlageOnFinish"); e != nil {
		return gv, e
	}
	sym, err = g.plugin.Lookup("VorlageShutdown")
	if err == nil {
		g.vorlageShutdown, ok = sym.(func() error)
	}
	if e := goProchandleerr(err, ok, "VorlageShutdown"); e != nil {
		return gv, e
	}
	// good link for v1
	return []*goProc{&g}, nil
}

func (g goProc) Startup() (vorlageproc.ProcessorInfo, error) {
	r, err := g.vorlageStartup()
	if err != nil {
		return r, err
	}
	return r, nil
}
func (g goProc) OnRequest(info vorlageproc.RequestInfo, i *interface{}) []vorlageproc.Action {
	return g.vorlageOnRequest(info, i)
}
func (g goProc) DefineVariable(info vorlageproc.DefineInfo, i interface{}) vorlageproc.Definition {
	return g.vorlageDefineVariable(info, i)
}
func (g goProc) OnFinish(info vorlageproc.RequestInfo, i interface{}) {
	g.vorlageOnFinish(info, i)
}
func (g goProc) Shutdown() error {
	return g.vorlageShutdown()
}

var _ vorlageproc.Processor = goProc{}

// reloadindex can be nil
// reloadindex will channel in indexes from this returned array that need to
// be reloaded because they were changed.
// If you want to shut the watcher down, just jam a -1 in that channel
func loadGoProcessors(dir string) ([]*goProc, error) {
	var procs []*goProc
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !validGoProcName(f.Name()) {
			continue
		}

		path := dir + "/" + f.Name()
		if dir == "" {
			path = f.Name()
		}

		p, err := loadGoProc(path, f.Name())
		if err != nil {
			return procs, lmerrorNew(0x19945,
				"failed to load go library",
				err,
				"",
				path)
		}
		p = append(p, preLoadGo...)
		var pconv = make([]*goProc, len(p))
		for i := range pconv {
			pconv[i] = p[i]
		}
		procs = append(procs, pconv...)
		Logger.Debugf("loaded golang elf %s from %s (%d processors)", f.Name(), path, len(p))
	}
	return procs, nil
}

func validGoProcName(fname string) bool {
	libnames := goLibraryFilenameSig.FindStringSubmatch(fname)
	if libnames == nil {
		Logger.Debugf("%s - not valid name format to be considered as golang processor", fname)
		return false
	}
	return true
}

func (c *Compiler) watchGoPath(path string) {
	w, err := newwatcher(path)
	if err != nil {
		Logger.Alertf("watcher failed: %s", err)
		return
	}
	c.gowatcher = &w
	defer w.close()
	var filename string
	for {
		filename, err = w.waitForUpdate()
		if err != nil {
			Logger.Alertf("watcher failed to wait for update (will be closing): %s", err)
			w.closederr = err
			w.closed = true
			return
		}
		// a file was just closed from being written too...

		// get the file's info
		fullpath := path + "/" + filename
		stat, err := os.Stat(fullpath)
		if err != nil {
			Logger.Noticef("auto-reload detected file %s in %s but failed to get a stat on it, skipping.", filename, path)
			continue
		}
		// is it a directory?
		if stat.IsDir() {
			Logger.Debugf("new file %s placed in %s is a directory, auto-reload doing nothing.", filename, path)
			continue
		}

		// is it a valid name?
		if !validGoProcName(filename) {
			continue
		}

		// okay so at this point we know they just moved in / replaced a file
		// that is attempting to be a valid processor.
		newprocs, err := loadGoProc(fullpath, filename)
		if err != nil {
			Logger.Errorf("auto-detect failed to load new go processor: %s", err)
			continue
		}

		// and now we know it IS a valid processor, go forth update it.
		Logger.Infof("new valid processor detected (%s)", fullpath)
		err = c.updategoproc(filename, newprocs)
		if err != nil {
			Logger.Alertf("watcher failed to wait for update (will be closing): %s", err)
			w.closederr = err
			w.closed = true
			return
		}

	}
}

func (c *Compiler) updategoproc(filename string, newprocs []*goProc) (err error) {

	// lets begin to stall new compiles until we get this thing loaded in.
	c.makestall(4)
	// and when everything is sorted out, remove the stall.
	defer c.cont()

	// was this new file replacing an old one that had previously gave us
	// processors?
	for i := range c.goprocessors {
		if c.goprocessors[i].sourcefile == filename {
			// set them to be deleted. they are outdated.
			c.goprocessors[i] = nil
		}
	}

	// add the new processors
	c.goprocessors = append(c.goprocessors, newprocs...)

	return c.rebuildProcessors()
}
