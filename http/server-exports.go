package http

import "net/http"
import ".."

type Request struct {
	w http.ResponseWriter
	r *http.Request
}

var currentConnectionPool map[doccomp.Rid]Request

// returns nil if request
func GetRequestEditor(rid doccomp.Rid) *Request {
	r, ok := currentConnectionPool[rid]
	if !ok {
		return nil
	}
	return &r
}

// a definition that's calling this must be before any content is outputted.
func (r *Request) SetCookie(cookie *http.Cookie) {
	http.SetCookie(r.w, cookie)
}
