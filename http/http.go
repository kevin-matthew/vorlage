package vorhttp

import (
	vorlage ".."
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"syscall"
)

// config var set on build
var buildVersion string
var buildHash string

var DocumentRoot string = "/var/www"
var BindAddress string = "localhost:80"
var UseFcgi bool = false
var ConfigFile = "/etc/vorlage/http.conf"
var TLSPrivateKey = ""
var TLSPublicKey = ""
var reloadProcessors = true

var config = []ConfigBinding{
	{
		Name:        "http-documentroot",
		Description: "the document root where the server will run at",
		VarAddress:  &DocumentRoot,
	},
	{
		Name:        "http-bindaddress",
		Description: "the address that vorlage will bind onto",
		VarAddress:  &BindAddress,
	},
	{
		Name:        "http-usefcgi",
		Description: "if true, vorlage will bind to http-bindaddress as an fcgi application.",
		VarAddress:  &UseFcgi,
	},
	{
		Name:        "http-buffer-multipart",
		Description: "The maximum memory allocated during multipart requests.",
		VarAddress:  &MultipartMaxMemory,
	},
	{
		Name: "http-tls-private-key",
		Description: `location of the private key for the use of https. leave blank to disable https.
If you wish to enable https, make sure your http-bindaddress specifies :443 as the port.
If http-usefcgi is enabled, this is ignored.`,
		VarAddress: &TLSPrivateKey,
	},
	{
		Name:        "http-tls-public-key",
		Description: "location of the public key for the use of https, if http-tls-private-key is empty this will be ignored.",
		VarAddress:  &TLSPublicKey,
	},
	//{
	//	Name: "vorlage-buffer",
	//	Description: "The size of the buffer that is streamed from the disk through vorlage per request.",
	//	VarAddress: &ProcessingBufferSize,
	//},
	{
		Name:        "vorlage-ldpath",
		Description: "A path to a directory to which vorlage will search for available vorlageprocs.",
		VarAddress:  &vorlage.CLoadPath,
	},
	{
		Name:        "vorlage-goldpath",
		Description: "A path to a directory to which vorlage will search for available go vorlageprocs.",
		VarAddress:  &vorlage.GoPluginLoadPath,
	},
	{
		Name:        "vorlage-reload-processors",
		Description: "If true, then vorlage will automatically re-load processors if it detects the file has changed. Use only for debugging and developing.",
		VarAddress:  &reloadProcessors,
	},
	{
		Name:        "log-debug",
		Description: "If set, will output debug information to the file. Note that outputting debug information must only be done when, well, debugging. Enabling debugging may cause dramatic slow downs.",
		VarAddress:  &logs.Debug,
	},
	{
		Name:        "log-verbose",
		Description: "If set, will output verbose information to the selected file.",
		VarAddress:  &logs.Verbose,
	},
	{
		Name:        "log-warnings",
		Description: "If set, will output warnings to the selected file. An warnings constitutes any behaviour that can lead to errors and/or failures.",
		VarAddress:  &logs.Warnings,
	},
	{
		Name:        "log-errors",
		Description: "If set, will output errors to the selected file. An error constitutes any unintended behaviour that was caused by user input.",
		VarAddress:  &logs.Errors,
	},
	{
		Name:        "log-failures",
		Description: "If set, will output failures to the selected file. A failure constitutes any unintended behaviour that wasn't caused by the user input. A failure can also be referred to as a bug.",
		VarAddress:  &logs.Failures,
	},
	{
		Name:        "log-timestamps",
		Description: "If true, log files will be given timestamps on each entry. Useless when debugging, really useful when going live.",
		VarAddress:  &logs.Timestamps,
	},
	{
		Name:        "extensions",
		Description: "The list of valid extensions to which vorlage will compile",
		VarAddress:  &FileExt,
	},
	{
		Name:        "tryfiles",
		Description: "A list of file names that vorlage will look for when a directory is requested",
		VarAddress:  &TryFiles,
	},
	{
		Name: "blocking-regexp",
		Description: `A perl-style regular expression that if matches the requested path, will not serve. For example, if you wanted to prevent all files under '/mystuff' from being served, set this equal to '^/mystuff'.
By default, this is set to "/(\.[^/.]+|\.\.[^/]+)", which means it will block all files that either start with '.' and/or are a decedent of a folder that starts with a '.'`,
		VarAddress: &BlockedFilesRegexp,
	},
}

var mainlogContext logcontext
var httplogContext logcontext

func Main() {

	// configure
	if err := BindAll(config); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, `failed to read configuring: %s
`, err)
		os.Exit(1)
	}

	// --help and --version (GNU standard)
	if len(os.Args) == 2 && os.Args[1] == "--help" {
		_, _ = fmt.Printf("usage: %s [--ARGUMENT=VALUE]... [CONFIG_FILE]\n", os.Args[0])
		_, _ = fmt.Printf(`       %s --help
`, os.Args[0])
		_, _ = fmt.Printf(`       %s --version
`, os.Args[0])
		_, _ = fmt.Printf(`       %s --default-conf

`, os.Args[0])
		fmt.Printf("Valid --ARGUMENT=VALUE pairs:\n")
		// load the config file so that help menu shows the default values after
		// the configure file has been loaded.
		_ = LoadConfFile(ConfigFile)
		_, _ = fmt.Fprintf(os.Stdout, HelpArgs())
		_, _ = fmt.Fprintf(os.Stdout, "Note: The above arguments can be pre-set in the CONFIG_FILE\n")
		_, _ = fmt.Fprintf(os.Stdout, "      as ARGUMENT=VALUE pairs.\n")
		_, _ = fmt.Fprintf(os.Stdout, "Note: The default CONFIG_FILE location is %s\n", ConfigFile)
		os.Exit(0)
	}
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		_, _ = fmt.Printf(`vorlage %s (build %s)
Copyright (c) 2021 Ellem Inc., all rights reserved.
Full license at https://www.ellem.ai/vorlage/license.html
`, buildVersion, buildHash)
		os.Exit(0)
	}

	// --default-conf (prints out the default configuration file)
	if len(os.Args) == 2 && os.Args[1] == "--default-conf" {
		_, _ = fmt.Printf("%s", HelpFile())
		os.Exit(0)
	}

	// if there is at least 1 argument, that is the configuration file.
	// so load that one instead of the default configuration
	params := GetParameters(os.Args[1:])
	if len(params) == 1 {
		ConfigFile = params[0]
	}
	if err := LoadConfFile(ConfigFile); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, err.Error()+"\n")
		_, _ = fmt.Fprintf(os.Stderr, "See "+os.Args[0]+" --help\n")
		os.Exit(1)
	}

	// now parse in the args ontop of the configuration file
	if err := LoadConfArgs(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, err.Error()+"\n")
		_, _ = fmt.Fprintf(os.Stderr, "See "+os.Args[0]+" --help\n")
		os.Exit(1)
	}

	// configuration complete.
	// now lets set up our logging
	if err := logs.LoadChannels(); err != nil {
		_, _ = fmt.Fprint(os.Stderr, "failed to open log file: "+err.Error()+"\n")
		os.Exit(1)
	}
	mainlogContext = logcontext{
		context: "Main",
		c:       &logs,
	}
	httplogContext = logcontext{
		context: "http",
		c:       &logs,
	}

	// bind to the address we'll be using for http request
	mainlogContext.Infof("binding to address %s...", BindAddress)
	l, err := net.Listen("tcp", BindAddress)
	if err != nil {
		errmsg := fmt.Sprintf("failed to bind to address %s: %s", BindAddress, err)
		mainlogContext.Errorf("%s", errmsg)
		err2 := sdError(syscall.ENOTCONN, errmsg)
		if err2 != nil {
			mainlogContext.Noticef("failed to update systemd status: %s", err2.Error())
		}
		os.Exit(1)
	}

	// set up the vorlage logging
	vorlagelogcontext := logcontext{
		context: "vorlage",
		c:       &logs,
	}
	vorlage.Logger = vorlagelogcontext

	// build up the compiler
	c, err := vorlage.NewCompiler()
	if err != nil {
		errmsg := fmt.Sprintf("failed to load go plugin: %s", err)
		mainlogContext.Errorf(errmsg)
		err2 := sdError(syscall.ELIBEXEC, errmsg)
		if err2 != nil {
			mainlogContext.Noticef("failed to update systemd status: %s", err2.Error())
		}
		os.Exit(1)
		return
	}

	// Load the TLS files if present
	// This step is largely redundant. As the LoadX509KeyPair will be called
	// from the serve function. We do this now to see if we have any errors
	// as to report back earlier than later.
	// We report any TLS errors before sdReady.
	if TLSPrivateKey != "" {
		_, err = tls.LoadX509KeyPair(TLSPublicKey, TLSPrivateKey)
		if err != nil {
			errmsg := fmt.Sprintf("failed to load TLS X509 key pair: %s", err)
			mainlogContext.Errorf(errmsg)
			err2 := sdError(syscall.ENOENT, errmsg)
			if err2 != nil {
				mainlogContext.Noticef("failed to update systemd status: %s", err2.Error())
			}
			os.Exit(1)
			return
		}
	}

	// all the hard setup work is done.

	// set up signals to listen too.
	// before we start the server, listen to common signals
	var shutdown bool
	var shutdownmu sync.Mutex
	go func() {
		sc := make(chan os.Signal, 1)
		// TODO: SIGHUP
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM /*syscall.SIGHUP,*/, os.Interrupt)
		sig := <-sc
		switch sig {
		/*case syscall.SIGHUP:
		// reload
		mainlogContext.Debugf("signal %s received, reloading", sig.String())

		// let systemd know we're reloading
		errT = sdReloading()
		if errT != nil {
			mainlogContext.Noticef("%s", errT)
		}

		// TODO reload logic.

		return*/

		case syscall.SIGINT:
			fallthrough
		case syscall.SIGTERM:
			// shutdown
			mainlogContext.Debugf("signal %s received, shutting down", sig.String())

			// let systemd know we're stopping
			err = sdStopping()
			if err != nil {
				mainlogContext.Noticef("%s", err)
			}
			shutdownmu.Lock()
			shutdown = true
			shutdownmu.Unlock()
			// got a signal, shut 'er down.
			_ = l.Close()
			return
		}
	}()

	// if they supplied a blocking regexep, compile it
	var blockingregexp *regexp.Regexp
	if BlockedFilesRegexp != "" {
		blockingregexp, err = regexp.Compile(BlockedFilesRegexp)
		if err != nil {
			errmsg := fmt.Sprintf("failed to compile blocking regexp: %s", err)
			mainlogContext.Errorf(errmsg)
			err2 := sdError(0x38193, errmsg)
			if err2 != nil {
				mainlogContext.Noticef("failed to update systemd status: %s", err2.Error())
			}
			os.Exit(1)
			return
		}
	}

	// start the server
	var srvmsg string = "Serving "
	if UseFcgi == true {
		srvmsg += "FCGI requests "
	} else if TLSPrivateKey != "" {
		srvmsg += "HTTPS requests "
	} else {
		srvmsg += "HTTP requests "
	}
	srvmsg += "out of " + DocumentRoot

	mainlogContext.Infof(srvmsg)
	err2 := sdReady(srvmsg, uint64(os.Getpid()))
	if err2 != nil {
		mainlogContext.Noticef("failed to update systemd status: %s", err2.Error())
	}

	err = Serve(l, UseFcgi, DocumentRoot, c, blockingregexp, TLSPrivateKey, TLSPublicKey)
	shutdownmu.Lock()
	if shutdown {
		shutdownmu.Unlock()
		mainlogContext.Infof("shutting down peacefully")
		os.Exit(0)
	} else {
		shutdownmu.Unlock()
		var errmsg string
		if err != nil {
			errmsg = fmt.Sprintf("http server exited unexpected with error: %s", err)
		} else {
			errmsg = fmt.Sprintf("http server exited unexpectedly without error")
		}
		mainlogContext.Errorf(errmsg)
		err2 := sdError(syscall.ENETDOWN, errmsg)
		if err2 != nil {
			mainlogContext.Noticef("failed to update systemd status: %s", err2.Error())
		}
		os.Exit(1)
	}
	return
}
