package http

import (
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"strings"
)

import doccomp ".."

type handler struct {
	docroot  string
	compiler doccomp.Compiler
}

func (h handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {

	// transversal attacks
	if BlockTransversalAttack {
		if isUpwardTransversal(request.URL.Path) {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	var fileToUse = h.docroot + request.URL.Path

	// does this file exist at all?
	stat, err := os.Stat(fileToUse)
	if err != nil {
		if os.IsNotExist(err) {
			writer.WriteHeader(http.StatusNotFound)
			println("could not find " + fileToUse)
			return
		}
		if os.IsPermission(err) {
			writer.WriteHeader(http.StatusForbidden)
			return
		}
		writer.WriteHeader(http.StatusBadRequest)
		h.writeError(err)
		return
	}

	// if we hit a directory, add an existing 'tryfile' to the path
	if stat.IsDir() {
		var i int
		for i = 0; i < len(TryFiles); i++ {
			// make sure we don't add an extra '/' if it's already there.
			if request.URL.Path[len(request.URL.Path)-1] == '/' {
				fileToUse = h.docroot + request.URL.Path + TryFiles[i]
			} else {
				fileToUse = h.docroot + request.URL.Path + "/" + TryFiles[i]
			}

			// check the stat of the path+tryfile to see if we have an
			// existing one
			stat, err = os.Stat(fileToUse)
			if err != nil {
				if os.IsNotExist(err) {
					// that tryfile doesn't exist, go to the next one.
					continue
				}
				// all other errors should be treated as normal.
				if os.IsPermission(err) {
					writer.WriteHeader(http.StatusForbidden)
					return
				}
				writer.WriteHeader(http.StatusBadRequest)
				h.writeError(err)
				return
			}
			// at this point we've found the tryfile to use for this directory
			break
		}
		if i == len(TryFiles) {
			// if i==len(TryFiles) that means we never found a tryfile.
			// so lets 404 em.
			writer.WriteHeader(http.StatusNotFound)
			return
		}
	}

	var stream io.ReadCloser
	var inputs map[string]string
	var streaminputs map[string]io.Reader
	var cookies []*http.Cookie
	var req doccomp.Request

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
		// we don't want to process this file... doesn't have an acceptable
		// extension
		stream, err = os.Open(fileToUse)
		if err != nil {
			if os.IsNotExist(err) {
				writer.WriteHeader(http.StatusNotFound)
				return
			}
			if os.IsPermission(err) {
				writer.WriteHeader(http.StatusForbidden)
				return
			}
			writer.WriteHeader(http.StatusBadRequest)
			h.writeError(err)
			return
		}
		goto writeStream
	}

	// parse the form and multipart form
	if err := request.ParseMultipartForm(MultipartMaxMemory); err != nil {
		if err != http.ErrNotMultipart {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(err.Error()))
			return
		}
	}

	// any request with multiple declarations of the same value is invalid.
	inputs = make(map[string]string)
	for k, s := range request.Form {
		if len(s) > 1 {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte("'" + k + "' contained multiple values"))
			return
		}
		if len(s) == 1 {
			inputs[k] = s[0]
		} else {
			inputs[k] = ""
		}
	}

	// do the same with the multipart form
	if request.MultipartForm != nil {
		streaminputs = make(map[string]io.Reader)
		for k, s := range request.MultipartForm.File {
			if len(s) != 1 {
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte("'" + k + "' contained multiple values or was empty"))
				return
			}

			file, err := request.MultipartForm.File[k][0].Open()
			if err != nil {
				writer.WriteHeader(http.StatusInternalServerError)
				_, _ = writer.Write([]byte("failed to open file '" + k + "'"))
				h.writeError(err)
				return
			}
			defer file.Close()
			streaminputs[k] = file
		}
	}

	// do the same with cookies
	cookies = request.Cookies()
	for i := range cookies {
		inputs[cookies[i].Name] = cookies[i].Value
	}

	// prepare the request in doccomp terms
	req = doccomp.Request{
		Filepath:    fileToUse,
		Input:       inputs,
		StreamInput: streaminputs,
	}
	// compile the document and get an Rid
	stream, err = h.compiler.Compile(&req)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte("failed to process document"))
		h.writeError(err)
		return
	}

	// now that we have the Rid, add everything in the Request pool.
	// be sure to de allocate when we're done writting to stream.
	addToConnectionPool(req.Rid, writer, request, stream)
	defer removeFromConnectionPool(req.Rid)

writeStream:
	// lets clear out some headers
	// content type
	extI := strings.LastIndex(fileToUse, ".")
	if extI != -1 {
		mimeT := mime.TypeByExtension(fileToUse[extI:])
		writer.Header().Add("Content-Type", mimeT)
	} else {
		writer.Header().Add("Content-Type", "application/octet-stream")
	}
	buff := make([]byte, ProcessingBufferSize)
	_, err = io.CopyBuffer(writer, stream, buff)
	_ = stream.Close()
	if err != nil {
		// cannot write headers here becauase we already wrote the
		// headers earlier.
		h.writeError(err)
		return
	}
	// at this point we've successfully found, processed, and served the file.
}

// thread safe
func addToConnectionPool(rid doccomp.Rid, writer http.ResponseWriter, r *http.Request, docstream io.ReadCloser) {
	connectionMu.Lock()
	currentConnectionPool[rid] = Request{writer, r, docstream}
	connectionMu.Unlock()
}

// thread safe
func removeFromConnectionPool(rid doccomp.Rid) {
	connectionMu.Lock()
	delete(currentConnectionPool, rid)
	connectionMu.Unlock()

}

func (h handler) writeError(err error) {
	println("vorlag-http error: " + err.Error())
}

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
 * net/http to handle all the http protocols and doccomp to handle the
 * putting-together of HTML documents.
 *
 * Make sure if your documentRoot will be local you use "."
 *
 * (confroming too: net/http/server.go)
 */
func Serve(l net.Listener, procs []doccomp.Processor, useFcgi bool, documentRoot string) error {

	c, err := doccomp.NewCompiler(procs)
	if err != nil {
		return err
	}

	currentConnectionPool = make(map[doccomp.Rid]Request)
	h := handler{
		docroot:  documentRoot,
		compiler: c,
	}
	if useFcgi {
		return fcgi.Serve(l, h)
	} else {
		return http.Serve(l, h)
	}
}
