package http


import (
	"crypto/rand"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)
import "../lmlog"

// must be a directory.
var SessionStoragePath = "."

// the time in which a session is deleted after the client leaves the site
var SessionLife = 60 * 60 * 48 // 48 hours

const CookieName = "DossibaySession"

type Session struct {
	id         string
	expireDate time.Time // set to time.
	// Time{} for it to expire when browser close
	filePath string
	Values   map[string]string
}

const sessionIdLength = 42

/******** sesErrors ********/
type sesError string

func (e sesError) Error() string {
	return string(e)
}
func NewErrorE(errStr sesError, because error) sesError {
	return sesError(errStr.Error() + ": " + because.Error())
}
func NewErrorS(errStr sesError, subject string) sesError {
	return sesError(errStr.Error() + " (" + subject + ")")
}

const ErrNoSession sesError = "no session"
const ErrBadSessionFile sesError = "bad session file"
const ErrSessionExists sesError = "session id already exists"
const ErrBadVariable sesError = "invalid session assignment"
const ErrInvalidSessionId sesError = "invalid session id"

/*
 * Attempts to create a new session
 */
func sessionCreate() (ses Session, err error) {
	i := 0
	// we'll generate a session ID, if it exists try again. After 3 times
	// give up and return a sessionexists error
	for i = 0; i < 3; i++ {
		ses.id = generateSessionId()
		if sessionExists(ses) != nil {
			continue
		}
		ses.filePath = SessionStoragePath + "/" + ses.id
		ses.Values = make(map[string]string)
		ses.expireDate = time.Now().Add(time.Second * time.Duration(SessionLife))
		err := ses.Save()
		if err != nil {
			return ses, err
		}
		break
	}
	if i == 3 {
		return ses, ErrSessionExists
	}
	return ses, nil
}

func generateSessionId() string {
	ret := make([]byte, sessionIdLength)
	_, _ = rand.Read(ret)
	return string(hex.EncodeToString(ret))
}

// checks to see if a session already exists or not
func sessionExists(session Session) error {
	_, err := os.Stat(session.filePath)
	if os.IsNotExist(err) {
		return nil
	}
	return ErrSessionExists
}

// loads the session from the given sessionId
func sessionLoadFromId(sessionId string, doReload bool) (ses Session, err error) {
	bytes, err := hex.DecodeString(sessionId)
	for err != nil || len(bytes) != sessionIdLength {
		return ses, ErrInvalidSessionId
	}
	ses.id = sessionId
	ses.filePath = SessionStoragePath + "/" + sessionId
	if doReload {
		return ses, ses.Reload()
	} else {
		return ses, nil
	}
}

// returns nil if no cookie was found
func loadSessionCookie(r *http.Request) (sessionId *http.Cookie) {
	var sessionCookie *http.Cookie
	for _, c := range r.Cookies() {
		if c.Name == CookieName {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		return nil
	}
	return sessionCookie
}

/*
 * Returns error ErrNoSession or ErrInvalidSessionId if no valid sessionID was
 * sent by the client. Any other errors will be because of the file system
 * operations
 */
func sessionLoad(r *http.Request) (ses Session, err error) {
	cookie := loadSessionCookie(r)
	if cookie == nil {
		return ses, ErrNoSession
	}
	ses, err = sessionLoadFromId(cookie.Value, true)
	if err != nil {
		return
	}

	// renew the session
	ses.expireDate = time.Now().Add(time.Second * time.Duration(SessionLife))
	_ = ses.Save()
	return
}

/*
 * Will attempt to first load an existing session, and if that failed due to
 * ErrInvalidSessionId or ErrNoSession, then a new session is created (
 * and sent).
 */
func sessionStart(w http.ResponseWriter, r *http.Request) (ses Session, err error) {
	ses, err = sessionLoad(r)
	// if the session does not exists or was invalid, then just create a
	// new one.
	if err == ErrNoSession || err == ErrInvalidSessionId {
		ses, err = sessionCreate()
		if err != nil {
			return ses, err
		}
		http.SetCookie(w, ses.ToCookie(r))
	} else if err != nil {
		return ses, err
	}
	return ses, nil
}

// installs the session into the browser
func sessionInstall(w http.ResponseWriter, session Session) {
	http.SetCookie(w, session.ToCookie(nil))
}

/*
 * Destroys the session if it exsist in the http request. If there was
 * no session to begin with, nothing is done and no error is returned.
 */
func sessionDestroy(w http.ResponseWriter, r *http.Request) error {
	cookie := loadSessionCookie(r)

	// normal error handling. Except if there was no session to start with
	// then don't worry, because our goal is to have ErrNoSession
	if cookie == nil {
		return nil
	}
	cookie.Expires = time.Unix(0, 0)
	http.SetCookie(w, cookie)

	ses, err := sessionLoadFromId(cookie.Value, false)
	if err != nil {
		// just ignore the errors.
		// if they supplied an invlaid session id then oh well. We're deleting
		// it anyways.
		return nil
	}
	return ses.Delete()
}

/*
 * Will always return non-nil pointer to a cookie
 */
func (ses Session) ToCookie(r *http.Request) *http.Cookie {
	if r != nil {
		// if TLS is nil, its an http connection
		if r.TLS == nil {
			return &http.Cookie{
				Name:       CookieName,
				Value:      ses.id,
				Path:       "",
				Domain:     "",
				Expires:    ses.expireDate,
				RawExpires: "",
				MaxAge:     0,
				Secure:     false,
				HttpOnly:   true,
				SameSite:   0,
				Raw:        "",
				Unparsed:   nil,
			}
		}
	}

	return &http.Cookie{
		Name:       CookieName,
		Value:      ses.id,
		Path:       "",
		Domain:     "",
		Expires:    ses.expireDate,
		RawExpires: "",
		MaxAge:     0,
		Secure:     true,
		HttpOnly:   true,
		SameSite:   0,
		Raw:        "",
		Unparsed:   nil,
	}
}

/*
 * Reloads the session from the file system into memory.
 */
func (ses *Session) Reload() (err error) {
	content, err := ioutil.ReadFile(ses.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			lmlog.DebugF("session file does not exist: " + ses.filePath)
			return ErrNoSession
		}
		return err
	}

	lines := strings.Split(string(content), decDelimiter)
	if len(lines) < 1 {
		lmlog.Error("loading session file has no lines: " + ses.filePath)
		return ErrBadSessionFile
	}

	ses.expireDate, err = time.Parse(time.RFC3339, lines[0])
	if err != nil {
		lmlog.Error("loading session value has bad time format: " + err.Error())
		return ErrBadSessionFile
	}

	if ses.expireDate.Before(time.Now()) {
		lmlog.DebugF("session is expired, will delete: " + ses.filePath)
		_ = ses.Delete()
		return ErrNoSession
	}

	ses.Values = make(map[string]string)
	for i := 1; i < len(lines)-1; i++ {
		l := lines[i]
		parts := strings.SplitN(l, varDelimiter, 2)
		if len(parts) < 2 {
			lmlog.Error(string(ErrBadSessionFile) + " f" + l)
			return ErrBadSessionFile
		}
		ses.Values[parts[0]] = parts[1]
	}
	return nil
}

/*
 * Saves the session into the file system.
 */
func (ses Session) Save() (err error) {
	f, err := os.OpenFile(ses.filePath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE,
		0660)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write([]byte(ses.expireDate.Format(time.
		RFC3339) + decDelimiter)); err != nil {
		return err
	}
	for l, r := range ses.Values {
		if strings.IndexAny(l, decDelimiter+varDelimiter) != -1 ||
			strings.IndexAny(r, decDelimiter) != -1 {
			return NewErrorS(ErrBadVariable, l+"="+r)
		}
		str := l + varDelimiter + r + decDelimiter
		if _, err = f.Write([]byte(str)); err != nil {
			return err
		}
	}
	return nil
}

const decDelimiter = "\x03"
const varDelimiter = "\x02"

/*
 * Deletes the session from the filesystem, preventing re-use. This does NOT
 * prevent the browser from using this again. Use sessionDelete for that
 */
func (ses Session) Delete() (err error) {
	return os.Remove(ses.filePath)
}

/*
 * Gets the Id of the session. There's no 'setter' for this field
 * because the format of the session id is generated sequentially.
 */
func (ses Session) GetId() string {
	return ses.id
}

/*
 * Deletes all expired and bad session. returns number of sessions deleted
 */
func DeleteExpiredSessions() (err error) {
	sessionFiles, err := ioutil.ReadDir(SessionStoragePath)
	if err != nil {
		return err
	}
	var ses Session
	for _, s := range sessionFiles {
		ses.filePath = SessionStoragePath + "/" + s.Name()

		// delete all expired and invalid sessions
		err := ses.Reload()
		if err != nil && err != ErrNoSession {
			_ = ses.Delete()
		}
	}
	return nil
}

