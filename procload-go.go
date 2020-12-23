package vorlage

import (
	"./lmgo/errors"
	"./vorlage-interface/golang/vorlageproc"
	"io/ioutil"
	"plugin"
)

type goProc struct {
	plugin *plugin.Plugin
	libname string
	vorlageStartup   func() vorlageproc.ProcessorInfo
	vorlageOnRequest func(info vorlageproc.RequestInfo, i *interface{}) []vorlageproc.Action
	vorlageDefineVariable func(info vorlageproc.DefineInfo, i interface{}) vorlageproc.Definition
	vorlageOnFinish func(info vorlageproc.RequestInfo, i interface{})
	vorlageShutdown func() error

}

func goProchandleerr(err error, ok bool, s string) error {
	if !ok {
		return errors.New(0x153b41,
			"symbol not found",
			nil,
			"ensure the processor was properly built and is up to date",
			s)
	}
	if err != nil {
		return errors.New(0x153b42,
			"failed to link symbol",
			err,
			"ensure the processor was properly built and is up to date",
			s)
	}
	return nil
}

func loadGoProc(path string) (g goProc, err error) {
	g.plugin, err = plugin.Open(path)
	if err != nil {
		return g, errors.New(3185,
			"failed to open plugin file",
			err,
			"make sure the file is valid",
			path)
	}
	var ok bool
	var sym plugin.Symbol
	sym,err = g.plugin.Lookup("VorlageStartup")
	g.vorlageStartup,ok = sym.(func() vorlageproc.ProcessorInfo)
	if e := goProchandleerr(err,ok,"VorlageStartup"); e != nil {
		return g,e
	}
	sym,err = g.plugin.Lookup("VorlageOnRequest")
	g.vorlageOnRequest,ok = sym.(func(info vorlageproc.RequestInfo, i *interface{}) []vorlageproc.Action)
	if e := goProchandleerr(err,ok,"VorlageOnRequest"); e != nil {
		return g,e
	}
	sym,err = g.plugin.Lookup("VorlageDefineVariable")
	g.vorlageDefineVariable,ok = sym.(func(info vorlageproc.DefineInfo, i interface{}) vorlageproc.Definition)
	if e := goProchandleerr(err,ok,"VorlageDefineVariable"); e != nil {
		return g,e
	}
	sym,err = g.plugin.Lookup("VorlageOnFinish")
	g.vorlageOnFinish,ok = sym.(func(info vorlageproc.RequestInfo, i interface{}))
	if e := goProchandleerr(err,ok,"VorlageOnFinish"); e != nil {
		return g,e
	}
	sym,err = g.plugin.Lookup("VorlageShutdown")
	g.vorlageShutdown,ok = sym.(func() error)
	if e := goProchandleerr(err,ok,"VorlageShutdown"); e != nil {
		return g,e
	}
	return g,nil
}

func (g goProc) Startup() vorlageproc.ProcessorInfo {
	r := g.vorlageStartup()
	r.Name = g.libname
	return r
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


func LoadGoProcessors() ([]vorlageproc.Processor, error) {
	var procs []vorlageproc.Processor
	files, err := ioutil.ReadDir(GoPluginLoadPath)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		libnames := goLibraryFilenameSig.FindStringSubmatch(f.Name())
		if libnames == nil {
			continue
		}
		path := GoPluginLoadPath + "/" + f.Name()
		if GoPluginLoadPath == "" {
			path = f.Name()
		}

		p,err := loadGoProc(path)
		if err != nil {
			return procs,errors.New(0x19945,
				"failed to load go library",
				err,
				"",
				path)
		}
		p.libname = libnames[1]
		procs = append(procs, p)
		Logger.Debugf("loaded go processor %s from %s", p.libname, path)
	}
	return procs, nil
}

