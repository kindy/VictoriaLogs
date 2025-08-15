package objectstorage

import (
	"os"
	"strings"

	libfs "github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/objectstorage/local"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/objectstorage/s3"
)

type FS interface {
	ReadFile(path string) ([]byte, error)
	Upload(path string) error
	MustMkdirFailIfExist(path string)
	MustMkdirIfNotExist(path string)
	MustGetFreeSpace() uint64
	MustClose(f *os.File)
	IsPathExist(path string) bool
	MustSyncPathAndParentDir(path string)
	MustReadDir(dir string) []os.DirEntry
	MustCreateFlockFile() *os.File
	MustRemoveDir(dir string)
	IsPartiallyRemovedDir(dir string) bool
	MustWriteAtomic(path string, data []byte, canOverwrite bool)
	IsDirOrSymlink(de os.DirEntry) bool
	MustSyncPath(path string)
	MustWriteSync(path string, data []byte)
	MustOpenReaderAt(path string) *libfs.ReaderAt
	MustHardLinkFiles(srcDir, dstDir string)
	MustFileSize(path string) uint64
	MustGetTotalSpace() uint64
}

// New returns new storage backend
func New(path string) FS {
	if len(path) == 0 {
		logger.Panicf("path is required for storage")
	}
	n := strings.Index(path, "://")
	if n < 0 {
		return local.New(path)
	}
	scheme := path[:n]
	dir := path[n+len("://"):]
	switch scheme {
	case "s3":
		n := strings.Index(dir, "/")
		if n < 0 {
			logger.Panicf("missing directory on the s3 bucket %q", dir)
		}
		bucket := dir[:n]
		dir = dir[n:]
		fs, err := s3.New(bucket, dir)
		if err != nil {
			logger.Panicf("cannot initialize connection to s3: %w", err)
		}
		return fs
	case "", "local", "fs":
		return local.New(path)
	default:
		logger.Panicf("unsupported scheme of path=%s, supported schemes: `s3://`, `local://`, `fs://`", path)
	}
	return nil
}
