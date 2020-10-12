package http

import (
	"io"
	"net/http"
	"sync"
	"time"
)
import ".."

type Request struct {
	w         http.ResponseWriter
	r         *http.Request
	docstream io.ReadCloser
}

var connectionMu sync.Mutex
var currentConnectionPool map[doccomp.Rid]Request

// returns nil if request
func GetRequestEditor(rid doccomp.Rid) *Request {
	r, ok := currentConnectionPool[rid]
	if !ok {
		return nil
	}
	return &r
}

func (r *Request) Redirect(dest string) {
	_ = r.docstream.Close()
	http.Redirect(r.w, r.r, dest, http.StatusSeeOther)
}

// a definition that's calling this must be before any content is outputted.
func (r *Request) SetCookie(cookieName string, value string) {

	c := http.Cookie{
		Name:       cookieName,
		Value:      value,
		Path:       "",
		Domain:     "",
		Expires:    time.Time{},
		RawExpires: "",
		MaxAge:     0,
		Secure:     false,
		HttpOnly:   false,
		SameSite:   0,
		Raw:        "",
		Unparsed:   nil,
	}

	http.SetCookie(r.w, &c)
}
