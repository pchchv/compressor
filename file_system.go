package compressor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DirFS allows access to a directory on the disk with a serial file system interface.
type DirFS string

// implicitDirInfo is a fs.FileInfo for an implicit directory
// (implicitDirEntry) value. This is used when the archive may
// not contain actual entries for a directory, but we need to
// pretend it exists so its contents can be discovered and
// traversed.
type implicitDirInfo struct {
	implicitDirEntry
}

// implicitDirEntry represents a directory that does
// not actually exist in the archive, but is inferred
// from the paths of actual files in the archive.
type implicitDirEntry struct {
	name string
}

// Open opens the named file.
func (f DirFS) Open(name string) (fs.File, error) {
	if err := f.checkName(name, "open"); err != nil {
		return nil, err
	}

	return os.Open(filepath.Join(string(f), name))
}

// ReadDir returns a listing of all the files in the named directory.
func (f DirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := f.checkName(name, "readdir"); err != nil {
		return nil, err
	}

	return os.ReadDir(filepath.Join(string(f), name))
}

// Stat returns info about the named file.
func (f DirFS) Stat(name string) (fs.FileInfo, error) {
	if err := f.checkName(name, "stat"); err != nil {
		return nil, err
	}

	return os.Stat(filepath.Join(string(f), name))
}

// Sub returns an FS corresponding to the subtree rooted at dir.
func (f DirFS) Sub(dir string) (fs.FS, error) {
	if err := f.checkName(dir, "sub"); err != nil {
		return nil, err
	}

	info, err := f.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	return DirFS(filepath.Join(string(f), dir)), nil
}

// checkName returns an error if name is not a valid path according to the io/fs package doc,
// with an additional hint taken from the standard os.dirFS.Open() implementation,
// which checks for invalid characters in Windows paths.
func (f DirFS) checkName(name, op string) error {
	if !fs.ValidPath(name) || runtime.GOOS == "windows" && strings.ContainsAny(name, `\:`) {
		return &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}

	return nil
}

func (e implicitDirEntry) Name() string {
	return e.name
}

func (implicitDirEntry) IsDir() bool {
	return true
}

func (implicitDirEntry) Type() fs.FileMode {
	return fs.ModeDir
}

func (e implicitDirEntry) Info() (fs.FileInfo, error) {
	return implicitDirInfo{e}, nil
}

func (d implicitDirInfo) Name() string {
	return d.name
}

func (implicitDirInfo) Size() int64 {
	return 0
}

func (d implicitDirInfo) Mode() fs.FileMode {
	return d.Type()
}

func (implicitDirInfo) ModTime() time.Time {
	return time.Time{}
}

func (implicitDirInfo) Sys() interface{} {
	return nil
}
