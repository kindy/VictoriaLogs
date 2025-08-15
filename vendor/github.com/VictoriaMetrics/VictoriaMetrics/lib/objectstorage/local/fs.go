package local

import (
	"os"
	"path/filepath"

	libfs "github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

type FS struct {
	path string
}

func New(path string) *FS {
	return &FS{
		path: path,
	}
}

func (fs *FS) MustGetTotalSpace() uint64 {
	return libfs.MustGetTotalSpace(fs.path)
}

func (fs *FS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(fs.getPath(path))
}

func (*FS) MustFileSize(path string) uint64 {
	return libfs.MustFileSize(path)
}

func (fs *FS) MustHardLinkFiles(srcDir, dstDir string) {
	libfs.MustHardLinkFiles(fs.getPath(srcDir), fs.getPath(dstDir))
}

func (fs *FS) getPath(p string) string {
	return filepath.Join(fs.path, p)
}

func (fs *FS) MustMkdirFailIfExist(dir string) {
	libfs.MustMkdirFailIfExist(fs.getPath(dir))
}

func (fs *FS) MustGetFreeSpace() uint64 {
	return libfs.MustGetFreeSpace(fs.path)
}

func (*FS) MustClose(f *os.File) {
	libfs.MustClose(f)
}

func (fs *FS) MustMkdirIfNotExist(dir string) {
	libfs.MustMkdirIfNotExist(fs.getPath(dir))
}

func (fs *FS) IsPathExist(p string) bool {
	return libfs.IsPathExist(fs.getPath(p))
}

func (fs *FS) MustSyncPathAndParentDir(p string) {
	libfs.MustSyncPathAndParentDir(fs.getPath(p))
}

func (fs *FS) MustReadDir(dir string) []os.DirEntry {
	return libfs.MustReadDir(fs.getPath(dir))
}

func (fs *FS) MustCreateFlockFile() *os.File {
	return libfs.MustCreateFlockFile(fs.path)
}

func (fs *FS) IsPartiallyRemovedDir(dir string) bool {
	return libfs.IsPartiallyRemovedDir(fs.getPath(dir))
}

func (fs *FS) MustRemoveDir(dir string) {
	libfs.MustRemoveDir(fs.getPath(dir))
}

func (fs *FS) MustWriteAtomic(p string, data []byte, overwrite bool) {
	libfs.MustWriteAtomic(fs.getPath(p), data, overwrite)
}

func (*FS) IsDirOrSymlink(de os.DirEntry) bool {
	return libfs.IsDirOrSymlink(de)
}

func (fs *FS) MustSyncPath(p string) {
	libfs.MustSyncPath(fs.getPath(p))
}

func (fs *FS) MustWriteSync(p string, data []byte) {
	libfs.MustWriteSync(fs.getPath(p), data)
}

func (fs *FS) MustOpenReaderAt(p string) *libfs.ReaderAt {
	return libfs.MustOpenReaderAt(fs.getPath(p))
}

func (fs *FS) Upload(p string) error {
	return nil
}
