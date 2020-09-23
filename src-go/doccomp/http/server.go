package http

import (
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

import doccomp ".."

type Handler struct {
	docroot string
}

func (h Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {

	// transversal attacks
	if BlockTransversalAttack {
		if isUpwardTransversal(request.URL.Path) {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	var tryFilesIndex = 0
	var fileToUse = request.URL.Path

	// does this file exist at all?
	stat, err := os.Stat(h.docroot + fileToUse)
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

	// if we hit a directory, add an existing 'tryfile' to the path
	if stat.IsDir() {
		var i int
		for i = 0; i < len(TryFiles); i++ {
			// make sure we don't add an extra '/' if it's already there.
			if request.URL.Path[len(request.URL.Path)-1] == '/' {
				fileToUse = request.URL.Path + TryFiles[tryFilesIndex]
			} else {
				fileToUse = request.URL.Path + "/" + TryFiles[tryFilesIndex]
			}

			// check the stat of the path+tryfile to see if we have an
			// existing one
			stat, err = os.Stat(h.docroot + fileToUse)
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

	// does it have the file extension we don't want?
	if len(fileToUse) < len(FileExt) ||
		fileToUse[len(fileToUse)-len(FileExt):] != FileExt {
		// If so, just serve it as a normal download.
		stream, err = os.Open(fileToUse)
		goto writeStream
	}

	// parse the form and multipart form
	if err := request.ParseMultipartForm(MultipartMaxMemory); err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(err.Error()))
		return
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

	// do the actual processing
	stream, err = doccomp.Process(request.URL.Path, inputs, streaminputs)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte("failed to process document"))
		h.writeError(err)
		return
	}

writeStream:
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

func (h Handler) writeError(err error) {

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
 *
 * (confroming too: net/http/server.go)
 */
func Serve(l net.Listener, documentRoot string) error {
	h := Handler{
		docroot: documentRoot,
	}
	return http.Serve(l, h)
}
