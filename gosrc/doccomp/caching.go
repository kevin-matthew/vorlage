package doccomp

type Cache interface {

	/*
	 * this is asked every request. If true is returned, a call to AddToCache
	 * will follow. If false is returned, a call to GetFromCache will follow.
	 * On error, neither is called.
	 */
	ShouldCache(path string) (bool, error)

	/*
	 * add a document to the cache. it should be able to be indexed by using
	 * it's path from d.GetFilePath
	 */
	AddToCache(d Document) error

	/*
	 * Load the document from the cache by using its path.
	 */
	GetFromCache(path string) (*Document, error)
}
