package vorhttp

import (
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
)

import vorlage "vorlage"
import vorlageproc "ellem.so/vorlageproc"

type handler struct {
	docroot        string
	compiler       *vorlage.Compiler
	blockingRegexp *regexp.Regexp
}

type actionhandler struct {
	writer  http.ResponseWriter
	request *http.Request
}

func (a actionhandler) ActionCritical(err error) {
	a.writer.WriteHeader(http.StatusInternalServerError)
	_, _ = a.writer.Write([]byte(err.Error()))
}

func (a actionhandler) ActionAccessFail(err error) {
	a.writer.WriteHeader(http.StatusUnauthorized)
	_, _ = a.writer.Write([]byte(err.Error()))
}

func (a actionhandler) ActionSee(path string) {
	http.Redirect(a.writer, a.request, path, http.StatusSeeOther)
}

func (a actionhandler) ActionHTTPHeader(header string) {
	parts := strings.SplitN(header, ":", 2)
	if len(parts) != 2 {
		println("vorlage-http: invalid header (thus ignoring): " + header)
		return
	}
	a.writer.Header().Add(parts[0], parts[1])
}

func auth() {

}

func (h handler) ListenToChanges() {

}

func (h handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {

	defer func() {
		if r := recover(); r != nil {
			httplogContext.Critf("panic recovered: %s (dumping to stderr)", r)
			_, _ = fmt.Fprintln(os.Stderr, string(debug.Stack()))
			panic(r)
		}
	}()

	// transversal attacks
	if BlockTransversalAttack {
		if isUpwardTransversal(request.URL.Path) {
			httplogContext.Warnf("%s - is upward transversal", request.URL.Path)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	httplogContext.Debugf("%s -> %s %s", request.RemoteAddr, request.Method, request.RequestURI)

	var fileToUse = h.docroot + request.URL.Path

	// does this file exist at all?
	stat, err := os.Stat(fileToUse)
	if err != nil {
		if os.IsNotExist(err) {
			writer.WriteHeader(http.StatusNotFound)
			httplogContext.Warnf("%s - file does not exist", fileToUse)
			return
		}
		if os.IsPermission(err) {
			writer.WriteHeader(http.StatusForbidden)
			httplogContext.Warnf("%s - vorlage failed to read due to having bad permissions: %s", fileToUse, err)
			return
		}
		writer.WriteHeader(http.StatusBadRequest)
		httplogContext.Warnf("%s - %s", fileToUse, err)
		return
	}

	// make sure the url path is not blacklisted.
	if h.blockingRegexp != nil && h.blockingRegexp.Match([]byte(request.URL.Path)) {
		writer.WriteHeader(http.StatusForbidden)
		httplogContext.Warnf("%s - vorlage failed to read due to having bad permissions: %s", fileToUse, err)
		return
	}

	// if we hit a directory, add an existing 'tryfile' to the path
	if stat.IsDir() {
		// if they had requested a directory but had not included a trailing '/'
		// then that's not supported as it breaks relative paths for imports.
		if request.URL.Path[len(request.URL.Path)-1] != '/' {
			httplogContext.Debugf("%s is a directory, redirecting to %s", request.URL.Path, request.URL.Path+"/")
			http.Redirect(writer, request, request.URL.Path+"/", http.StatusFound)
			return
		}
		var i int
		for i = 0; i < len(TryFiles); i++ {
			// we know at this point that request.URL.Path includes a trailing '/'
			// so lets just combine it all together.
			fileToUse = h.docroot + request.URL.Path + TryFiles[i]

			// check the stat of the path+tryfile to see if we have an
			// existing one
			stat, err = os.Stat(fileToUse)
			if err != nil {
				if os.IsNotExist(err) {
					httplogContext.Debugf("directory %s cannot use index %s: %s", request.URL.Path, TryFiles[i], err)
					// that tryfile doesn't exist, go to the next one.
					continue
				}
				// all other errors should be treated as normal.
				if os.IsPermission(err) {
					writer.WriteHeader(http.StatusForbidden)
					httplogContext.Warnf("%s - vorlage failed to read due to having bad permissions: %s", fileToUse, err)
					return
				}
				writer.WriteHeader(http.StatusBadRequest)
				httplogContext.Warnf("%s - %s", fileToUse, err)
				return
			}
			// at this point we've found the tryfile to use for this directory
			break
		}
		if i == len(TryFiles) {
			// if i==len(TryFiles) that means we never found a tryfile.
			// so lets 404 em.
			writer.WriteHeader(http.StatusNotFound)
			httplogContext.Warnf("%s - file does not exist", fileToUse)
			return
		}
		httplogContext.Debugf("directory %s can use %s as the index", request.URL.Path, TryFiles[i])
	}

	var stream io.ReadCloser
	var inputs map[string]string
	var streaminputs map[string]vorlageproc.StreamInput
	var cookies []*http.Cookie
	var cstat vorlage.CompileStatus
	defer func() {
		if stream != nil {
			stream.Close()
		}
	}()

	// does it have the file extension we don't want?

	var ei int
	var e string
	for ei = 0; ei < len(FileExt); ei++ {
		e = FileExt[ei]
		if len(fileToUse) >= len(e) &&
			fileToUse[len(fileToUse)-len(e):] == e {
			break
		}
	}
	if ei == len(FileExt) {
		httplogContext.Debugf("vorlage will not be compiling %s because its not of one of the following extensions: %v", fileToUse, FileExt)
		// we don't want to process this file... doesn't have an acceptable
		// extension
		stream, err = os.Open(fileToUse)
		if err != nil {
			if os.IsPermission(err) {
				writer.WriteHeader(http.StatusInternalServerError)
				httplogContext.Errorf("%s - vorlage failed to open due to bad permissions", fileToUse, err)
				return
			}
			writer.WriteHeader(http.StatusInternalServerError)
			httplogContext.Errorf("%s - vorlage failed to open: %s", fileToUse, err)
			return
		}

		// if we're reading strait from a file... let's optimize by telling the
		// browser not to re-download if it wasn't modified recently.
		stat, err := os.Stat(fileToUse)
		if err != nil {
			httplogContext.Noticef("%s - failed to stat (but still opened): %s", fileToUse, err)
		} else {
			writer.Header().Add("cache-control", "private")
			modtime := stat.ModTime()
			writer.Header().Add("last-modified", modtime.Format(time.RFC1123))
			lastmodifiedHeader := request.Header.Get("If-Modified-Since")
			if lastmodifiedHeader != "" {
				lastmod, _ := time.Parse(time.RFC1123, lastmodifiedHeader)
				// truncate down to the second.
				fmodtime := modtime.Truncate(time.Second)
				if !fmodtime.After(lastmod) {
					writer.WriteHeader(http.StatusNotModified)
					httplogContext.Debugf("%s - file hasn't been modified before the requested date (%s)", fileToUse, lastmodifiedHeader)
					return
				}
			}
		}

		goto writeStream
	}

	// parse the form and multipart form
	if err := request.ParseMultipartForm(MultipartMaxMemory); err != nil {
		if err != http.ErrNotMultipart {
			httplogContext.Warnf("failed to load multipart: %s", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// any request with multiple declarations of the same value is invalid.
	inputs = make(map[string]string)
	for k, s := range request.Form {
		if len(s) > 1 {
			httplogContext.Warnf("%s contains multiple values (%v)", k, s)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(s) == 1 {
			inputs[k] = s[0]
		} else {
			httplogContext.Debugf("the input %s didn't contain a value (will default to \"\")", k)
			inputs[k] = ""
		}
	}

	// do the same with the multipart form
	if request.MultipartForm != nil {
		streaminputs = make(map[string]vorlageproc.StreamInput)
		for k, s := range request.MultipartForm.File {
			if len(s) != 1 {
				httplogContext.Warnf("%s contains multiple streams or is empty", k)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}

			var file fupload
			file.size = uint64(request.MultipartForm.File[k][0].Size)
			file.multipartfile, err = request.MultipartForm.File[k][0].Open()
			if err != nil {
				httplogContext.Errorf("failed to open stream from %s: %s", k, err)
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer func(file fupload) { _ = file.Close() }(file)
			streaminputs[k] = file
		}
	}

	// do the same with cookies
	cookies = request.Cookies()
	for i := range cookies {
		inputs[cookies[i].Name] = cookies[i].Value
	}

	// lastly, we'll put in some of our server's inputs.
	inputs["HTTP_REMOTE_ADDR"] = request.RemoteAddr
	inputs["HTTP_DOCUMENT_ROOT"] = h.docroot
	if request.TLS == nil {
		inputs["HTTP_HTTPS"] = ""
	} else {
		inputs["HTTP_HTTPS"] = "true"
	}

	// compile the document and get an Rid
	stream, cstat = h.compiler.Compile(fileToUse, inputs, streaminputs, actionhandler{writer, request})
	if cstat.Err != nil {
		if cstat.WasProcessor {
			// don't do anything because actionhandler's interface was called
			// and the relevant function already sent the bad headers.
			//
			// We don't need to log the error either sense the processor-based
			// error is documented not the server's fault.
			// debug just in case.
			httplogContext.Debugf("processor set error %s: %s", fileToUse, cstat.Err)
		} else {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte("server failed to load document"))
			httplogContext.Errorf("vorlage failed to compile %s: %s", fileToUse, cstat.Err)
		}
		return
	}

	// now that we have the Rid, add everything in the RequestInfo pool.
	// be sure to de allocate when we're done writting to stream.
	//addToConnectionPool(req.Rid, writer, request, stream)
	//defer removeFromConnectionPool(req.Rid)

writeStream:
	// lets clear out some headers
	// content type
	if writer.Header().Get("Content-Type") == "" {
		extI := strings.LastIndex(fileToUse, ".")
		if extI != -1 {
			mimeT := mime.TypeByExtension(fileToUse[extI:])
			httplogContext.Debugf("determined %s is of %s mimetype", fileToUse, mimeT)
			writer.Header().Add("Content-Type", mimeT)
		} else {
			httplogContext.Debugf("determined %s is not a mimetype, assuming octet-stream", fileToUse)
			writer.Header().Add("Content-Type", "application/octet-stream")
		}
	}
	buff := make([]byte, ProcessingBufferSize)
	_, err = io.CopyBuffer(writer, stream, buff)
	if err != nil {
		// cannot write headers here becauase we already wrote the
		// headers earlier.
		httplogContext.Errorf("failed to fully output %s: %s", fileToUse, err)
		return
	}
}

type fupload struct {
	size          uint64
	multipartfile multipart.File
}

func (f fupload) Read(p []byte) (n int, err error) {
	return f.multipartfile.Read(p)
}

func (f fupload) Close() error {
	if f.multipartfile == nil {
		return nil
	}
	return f.multipartfile.Close()
}

func (f fupload) GetLen() (n uint64, err error) {
	return f.size, nil
}

var _ vorlageproc.StreamInput = fupload{}

/*
 * Returns true if path will upward transversal (aka transversal attack).
 * Returns false if the path does not contain a upward transversal.
 *
 * For example, these will contain an upward transversal:
 *     "/.."
 *     "/www/../.."
 *     "/../etc/passwd"
 *     "/www/../../etc/passwd"
 *     "/../../../../../../../../etc/passwd"
 *
 * And these would NOT contain upward transversal:
 *     "/"
 *     "/www/../www2"
 *     "/www/../www2/file"
 *
 */
func isUpwardTransversal(path string) bool {
	parts := strings.Split(path, string(os.PathSeparator))
	var transversal int
	for _, p := range parts {
		if p == ".." {
			// they went up a directory
			transversal--
		} else if p == "." || p == "" {
			// they stayed in the same directory (no transversal)
		} else {
			// they went down a directory
			transversal++
		}
		// if they're negative, that means they went above more directories than
		// they did go down.
		if transversal < 0 {
			return true
		}
	}
	return false
}

/*
 * Serve accepts incoming HTTP connections on listener l using
 * net/http to handle all the http protocols and vorlageproc to handle the
 * putting-together of HTML documents.
 *
 * Make sure if your documentRoot will be local you use "."
 *
 * (confroming too: net/http/server.go)
 */
// todo: need to move the tls logic outside of this function so that
//      those errors are not mixed up with socket errors.
func Serve(l net.Listener,
	useFcgi bool,
	documentRoot string,
	c *vorlage.Compiler,
	blockingregexp *regexp.Regexp,
	privkey,
	pubkey string) (err error) {

	h := handler{
		docroot:        documentRoot,
		compiler:       c,
		blockingRegexp: blockingregexp,
	}

	if useFcgi {
		// serve over fcgi
		err = fcgi.Serve(l, h)
	} else {
		// serve over TLS
		if privkey != "" {
			mainlogContext.Infof("Serving TLS")
			srvr := &http.Server{
				Handler: h,
				TLSConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					MaxVersion: tls.VersionTLS13,
				},
			}
			err = srvr.ServeTLS(l, pubkey, privkey)
		} else {
			// serve as normal http
			err = http.Serve(l, h)
		}
	}

	// when done, tell the processors that we're done.
	c.Shutdown()
	return err
}
