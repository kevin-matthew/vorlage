package http

/*
 * if false, Transversal request will be allowed.
 *
 * For Example:
 *    website.com/../../../.../../../../etc/passwd
 *
 * This should always be true
 */
var BlockTransversalAttack = true

/*
 * Maximum memory for multipart form per request.
 */
var MultipartMaxMemory int64 = 0x500000

/*
 * Maximum memory used for processing documents per request.
 */
var ProcessingBufferSize int64 = 0x10000

/*
 * Filenames to try if a directory is accessed. An empty array disables this.
 * You probably shouldn't modify this variable while you're serving pages.
 */
var TryFiles []string = []string{"index.html", "index.proc.html"}

/*
 * Proc indicator. The file extension to look for that will activate
 * the processing. Otherwise a normal file request will take place.
 */
var FileExt = ".proc.html"

/*
 * if SessionCookie is enabled, a HttpOnly cookie will be created with the value
 * of 32 random bytes (in the form of 64 random hex values) and will retain on
 * the user's browser indefinitely. It's value can be accessed via the input
 * of __SESSIONCOOKIE.
 * Session is the same thing except the cookie will have the "Session" flag and
 * can be accessed by __SESSION.
 * Having both enabled will result in 2 cookies.
 */
var SessionCookie = true
var Session = true
