package main

import (
	"ellem.so/vorlageproc"
)

func VorlageStartup() (vorlageproc.ProcessorInfo, error) {
	p := vorlageproc.ProcessorInfo{
		Name:        "PCI Complience Headers",
		Description: "Provides several headers that make the webserver cplient to PCI",
	}
	return p, nil
}
func VorlageOnRequest(r vorlageproc.RequestInfo, i *interface{}) []vorlageproc.Action {
	return []vorlageproc.Action{
		{
			vorlageproc.ActionHTTPHeader,
			[]byte("X-Frame-Options: SAMEORIGIN"),
		},
		{
			vorlageproc.ActionHTTPHeader,
			[]byte("X-XSS-Protection: 1; mode-block"),
		},
		{
			vorlageproc.ActionHTTPHeader,
			[]byte("Strict-Transport-Security: max-age=31536000; includeSubDomains"),
		},
		{
			vorlageproc.ActionHTTPHeader,
			[]byte("X-Content-Type-Options: nosniff"),
		},
		{
			vorlageproc.ActionHTTPHeader,
			[]byte("Content-Security-Policy: default-src 'self' 'unsafe-eval' *.gstatic.com *.googleapis.com *.evans-dixon.com 'unsafe-inline'"),
		},
	}
}
func VorlageDefineVariable(info vorlageproc.DefineInfo, i interface{}) vorlageproc.Definition {
	return nil
}
func VorlageOnFinish(vorlageproc.RequestInfo, interface{}) {
	return
}
func VorlageShutdown() error {
	return nil
}
