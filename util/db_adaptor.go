package util

type DbAdaptor interface {
	GetFileId(filepath string) (fid string, err error)
	GetFileFullPath(fid string) (filepath string, err error)
	SetPathMeta(path string, ttl string) (err error)
	CacheFilePath(filepath string, fid string, ttl string) (err error)
}
