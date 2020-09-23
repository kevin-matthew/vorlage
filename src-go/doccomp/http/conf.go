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
const FileExt = ".proc.html"
