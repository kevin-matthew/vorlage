
// +build !wasm,!js

package lmlog

import (
	"net/http"
	"net/url"
)

var AdminMessagingToken = ""
var AdminMessagingUser  = ""
var AdminMessagingUrl   = ""
var AdminMessaging      = false

func MessageAdmins(title string, message string) {
	if !AdminMessaging || len(AdminMessagingUrl) == 0 {
		return
	}
	formData := url.Values{
		"token":   {AdminMessagingToken},
		"user":    {AdminMessagingUser},
		"message": {message},
		"title":   {title},
	}
	_, _ = http.PostForm(AdminMessagingUrl, formData)
}

func MessageAdminsLowPriority(title string, message string) {
	if !AdminMessaging {
		return
	}
	formData := url.Values{
		"token":    {AdminMessagingToken},
		"user":     {AdminMessagingUser},
		"message":  {message},
		"priority": {"-1"},
		"title":    {title},
	}
	_, _ = http.PostForm(AdminMessagingUrl, formData)
}
