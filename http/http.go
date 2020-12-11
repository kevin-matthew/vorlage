package main

import (
	vorlage ".."
	"../lmgo/conf"
	"fmt"
	"net"
	"os"
)



var DocumentRoot string = "."
var BindAddress string  = "localhost:80"
var UseFcgi bool        = false
var ConfigFile = "/etc/vorlage/http.conf"

var config = []conf.ConfigBinding{
	{
		Name: "http-documentroot",
		Description: "the document root where the server will run at",
		VarAddress: &DocumentRoot,
	},
	{
		Name: "http-bindaddress",
		Description: "the address that vorlage will bind onto",
		VarAddress: &BindAddress,
	},
	{
		Name: "http-usefcgi",
		Description: "if true, vorlage will bind to http-bindaddress as an fcgi application. this is a long description because fuck you and fuck me we're all going to die and forgotten in 200 years.",
		VarAddress: &UseFcgi,
	},
	{
		Name: "http-buffer-multipart",
		Description: "The maximum memory allocated during multipart requests.",
		VarAddress: &MultipartMaxMemory,
	},
	{
		Name: "vorlage-buffer",
		Description: "The size of the buffer that is streamed from the disk through vorlage per request.",
		VarAddress: &ProcessingBufferSize,
	},
	{
		Name: "vorlage-ldpath",
		Description: "A path to a directory to which vorlage will search for available processors.",
		VarAddress: &vorlage.CLoadPath,
	},
	{
		Name: "log-debug",
		Description: "If set, will output debug information to the file. Note that outputting debug information must only be done when, well, debugging. Enabling debugging may cause dramatic slow downs.",
		VarAddress: &logs.Debug,
	},
	{
		Name: "log-verbose",
		Description: "If set, will output verbose information to the selected file.",
		VarAddress: &logs.Verbose,
	},
	{
		Name: "log-warnings",
		Description: "If set, will output warnings to the selected file. An warnings constitutes any behaviour that can lead to errors and/or failures.",
		VarAddress: &logs.Warnings,
	},
	{
		Name: "log-errors",
		Description: "If set, will output errors to the selected file. An error constitutes any unintended behaviour that was caused by user input.",
		VarAddress: &logs.Errors,
	},
	{
		Name: "log-failures",
		Description: "If set, will output failures to the selected file. A failure constitutes any unintended behaviour that wasn't caused by the user input. A failure can also be referred to as a bug.",
		VarAddress: &logs.Failures,
	},
}


var mainlogContext logcontext
var httplogContext logcontext
func main() {
	// configure
	if err := conf.BindAll(config); err != nil {
		_,_ = fmt.Fprintf(os.Stderr, "failed to start configuring: " + err.Error())
		os.Exit(1)
	}
	if len(os.Args) == 2 && os.Args[1] == "--help" {

		_,_  = fmt.Printf("usage: %s [--ARGUMENT=VALUE]... [CONFIG_FILE]\n", os.Args[0])
		fmt.Printf("Valid --ARGUMENT=VALUE pairs:\n")
		// load the config file so that help menu shows the default values after
		// the configure file has been loaded.
		_ = conf.LoadConfFile(ConfigFile)
		_,_ = fmt.Fprintf(os.Stdout, conf.HelpArgs())

		_,_ = fmt.Fprintf(os.Stderr, "Note: The above arguments can be pre-set in the CONFIG_FILE\n")
		_,_ = fmt.Fprintf(os.Stderr, "      as ARGUMENT=VALUE pairs.\n")
		_,_ = fmt.Fprintf(os.Stderr, "Note: The default CONFIG_FILE location is %s\n", ConfigFile)
		os.Exit(0)
	}

	if len(os.Args) == 2 && os.Args[1] == "--defaults" {

		_,_  = fmt.Printf("%s", conf.HelpFile())
		os.Exit(0)
	}

	params := conf.GetParameters(os.Args[1:])
	if len(params) == 1 {
		ConfigFile = params[0]
	}

	if err := conf.LoadConfFile(ConfigFile); err != nil {
		_,_ = fmt.Fprintf(os.Stderr, err.Error() + "\n")
		_,_ = fmt.Fprintf(os.Stderr, "See " + os.Args[0] + " --help\n")
		os.Exit(1)
	}

	if err := conf.LoadConfArgs(os.Args[1:]); err != nil {
		_,_ = fmt.Fprintf(os.Stderr, err.Error() + "\n")
		_,_ = fmt.Fprintf(os.Stderr, "See " + os.Args[0] + " --help\n")
		os.Exit(1)
	}
	// configuration complete.
	// now lets set up our logging
	if err := logs.LoadChannels(); err != nil {
		_,_ = fmt.Fprintf(os.Stderr, "failed to open log file: " + err.Error() + "\n")
		os.Exit(1)
	}
	mainlogContext = logcontext{
		context: "main",
		c:       &logs,
	}
	httplogContext = logcontext{
		context: "http",
		c: &logs,
	}
	mainlogContext.Infof("logs configured")


	// bind to the address we'll be using for http request
	mainlogContext.Infof("binding to address %s...", BindAddress)
	l, err := net.Listen("tcp", BindAddress)
	if err != nil {
		mainlogContext.Errorf("failed to bind to address %s: %s", BindAddress, err)
		os.Exit(1)
	}

	// set up the vorlage logging
	vorlagelogcontext := logcontext{
		context: "vorlage",
		c:       &logs,
	}
	vorlage.SetLogger(vorlagelogcontext)

	// load the c processors
	mainlogContext.Infof("loading ELF processors out of %s...", vorlage.CLoadPath)
	FileExt = append(FileExt, ".html")
	procs, err := vorlage.LoadCProcessors()
	if err != nil {
		mainlogContext.Errorf("failed to load ELF processors: %s", err.Error())
		os.Exit(1)
		return
	}

	// start the server
	mainlogContext.Infof("starting server for document root \"%s\"...", DocumentRoot)
	err = Serve(l, procs, UseFcgi, DocumentRoot)
	if err != nil {
		mainlogContext.Infof( "http server exited with error: %s", err.Error())
		os.Exit(1)
		return
	}
	mainlogContext.Infof( "vorlage http server closed without error")
}
