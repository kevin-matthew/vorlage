package http

import (
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"errors"
)

var signalChannel chan os.Signal
func handleSignals(daemon Daemon, pidFilePath string) {
	for sig := range signalChannel {
		Notice("received signal: " + sig.String())
		switch sig {
		case syscall.SIGHUP:
			err := daemon.Load()
			if err != nil {
				Emerg("Failed to reload: " + err.Error())
			}
			break
		case syscall.SIGTERM, syscall.SIGINT:
			err := daemon.Stop()
			if err != nil {
				Crit("SIGTERM failed: " + err.Error())
				_ = syscall.Unlink(pidFilePath)
				os.Exit(1)
			} else {
				_ = syscall.Unlink(pidFilePath)
				os.Exit(0)
			}
			return
		default:
			// do nothing
		}
	}
}

func fallBackOnPIDExists(pidFilePath string) error {
	var fd int
	var err error
	if fd,err = syscall.Open(pidFilePath, syscall.O_RDONLY, 0666); err != nil {
		return errors.New("cannot open pid file to discover if http is" +
			" already running" + err.Error())
	}

	buffer := make([]byte,255)
	n,err  := syscall.Read(fd,buffer)
	_ = syscall.Close(fd)
	if err != nil {
		return errors.New("cannot read pid file to discover if http is" +
			" already running: " + err.Error())
	}

	pid,err := strconv.Atoi(string(buffer[:n]))
	if err != nil {
		return errors.New("cannot interpret pid file to discover if http is" +
			" already running: " + err.Error())
	}
	err  = syscall.Kill(pid, 0)
	if err != nil {
		if err == syscall.ESRCH {
			Notice("old pid file found, " +
				"but no running process attached to it, attempting to delete")
			err2 := os.Remove(pidFilePath)
			if err2 != nil {
				return errors.New("failed to delete old (" +
					"unused) pid file: " + err2.Error())
			} else {
				return nil
			}
		} else {
			return errors.New("cannot detect proccess status from old pid file: " + err.
				Error())
		}
	}
	return errors.New("a http is already running with this PID")
}

func Daemonize(daemon Daemon, pidFilePath string) {

	// capture signals
	signalChannel = make(chan os.Signal, 1)
	signal.Notify(signalChannel)
	go handleSignals(daemon, pidFilePath)
	tryopened := false

	// create pid file
	tryopen:
	fd, err := syscall.Open(pidFilePath, syscall.O_WRONLY | syscall.O_CREAT | syscall.O_EXCL, 0666)
	if err != nil {
		if os.IsExist(err) {
			err2 := fallBackOnPIDExists(pidFilePath)
			if err2 != nil {
				Crit("Failed to create pid file '" + pidFilePath + "': " + err.
					Error() + "(" + err2.Error() + ")")
			} else if !tryopened {
				tryopened = true
				goto tryopen
			}
		} else {
			Crit("Failed to open pid file '"+pidFilePath+"': " + err.Error())
		}
		os.Exit(1)
		return
	}
	pidStr := strconv.Itoa(os.Getpid())
	_,err   = syscall.Write(fd, []byte(pidStr))
	if err != nil {
		Crit("Failed to write pid in file: " + err.Error())
		os.Exit(1)
		return
	}
	_ = syscall.Close(fd)

	// actually run the http
	Debug("Loading http...")
	err  = daemon.Load()
	if err != nil {
		Crit("Failed to load: "+ err.Error())
		_ = syscall.Unlink(pidFilePath)
		os.Exit(1)
	}

	Debug("Starting http...")
	if err := daemon.Start(); err != nil {
		Error("Daemon returned error: "+ err.Error())
		_ = syscall.Unlink(pidFilePath)
		os.Exit(1)
	}
	Notice("Graceful exit")
	_ = syscall.Unlink(pidFilePath)
	os.Exit(0)
}


type Status int
const (
	StatNew    Status = iota
	StatLoading
	StatLoaded
	StatStarting
	StatStarted
	StatStopping
	StatStopped
)


type Daemon interface {

	/*
	 * DESCRIPTION: Load() will load the configuration of the http (ie,
	 * /etc/ files). It is called before initial Start()
	 * as well as when a SIGHUP is recieved.
	 *
	 * THREAD SAFETY: Load MUST be thread-safe. It MAY be called multiple times,
	 * it will be called both before and after Start().
	 *
	 * RETURN: returning an error is critical on initial start up, all
	 * subsequent Load() calls that return errors
	 * WILL NOT be critical to the proccess of the http.
	 */
	Load()  error

	/*
	 * DESCRIPTION: Start() will start the http and will not return until the http needs to exit.
	 * In other words, Start() should be treated as the http's 'main' function.
	 *
	 * THREAD SAFETY: Start does not have to be thread safe. It will only ever be
	 * called on the main thread.
	 *
	 * RETURN: the only time error == nil is when the http has exited nicely (ie, via Stop()).
	 * Thus, random critical failures will result in Start returning an error.
	 */
	Start() error

	/*
	 * THREAD SAFETY: Stop MUST be thread-safe as it will likely be called on a seperate
	 * thread that listens for arbitrary signals.
	 *
	 * RETURN: an error WILL ONLY be returned is if the http
	 * refuses to stop for one reason or another. Thus, even if Stop() had broken something, but
	 * the http still stopped, an error will not be returned. Furthermore, if Stop()
	 * is being called even after the http has already been stopped, no error should be returned.
	 */
	Stop()  error

	/*
	 * THREAD SAFETY: MUST be thread-safe as it will likely be called on a seperate
	 * thread that listens for arbitrary signals.
	 *
	 * RETURN: Returns the status of the http
	 */
	//GetStatus() Status
}