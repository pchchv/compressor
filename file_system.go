package compressor

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DirFS allows access to a directory on the disk with a serial file system interface.
type DirFS string

// implicitDirInfo is a fs.FileInfo for an implicit directory (implicitDirEntry) value.
// This is used when the archive may not contain actual entries for a directory,
// but we need to pretend it exists so its contents can be discovered and traversed.
type implicitDirInfo struct {
	implicitDirEntry
}

// implicitDirEntry represents a directory that does not actually exist in the archive,
// but is inferred from the paths of actual files in the archive.
type implicitDirEntry struct {
	name string
}

// extractedFile implements fs.File, thus it represents an "opened" file,
// slightly different from the File type representing a file that can possibly be opened.
// If the file is actually opened,
// this type ensures that the parent archive is closed when this file from it is also closed.
type extractedFile struct {
	File

	// Set these fields if a "regular file" which has actual readable content.
	// ReadCloser should be the file's reader, and parentArchive is a reference to the archive the files comes out of.
	// If parentArchive is set, it will also be closed along with the file when Close() is called.
	io.ReadCloser
	parentArchive io.Closer
}

type fakeArchiveFile struct{}

// dirFile implements the fs.ReadDirFile interface.
type dirFile struct {
	extractedFile
	entries     []fs.DirEntry
	entriesRead int
}

// dirFileInfo is an implementation of fs.FileInfo that is only used for files that are directories.
// It always returns 0 size, directory bit set in the mode, and true for IsDir.
// It is often used as the FileInfo for dirFile values.
type dirFileInfo struct {
	fs.FileInfo
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

func (f fakeArchiveFile) Stat() (fs.FileInfo, error) {
	return implicitDirInfo{implicitDirEntry{name: "."}}, nil
}

func (f fakeArchiveFile) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (f fakeArchiveFile) Close() error {
	return nil
}

// Close closes the the current file if it is open and the parent archive if specified.
// For directories that do not specify these fields, this does not work.
func (ef extractedFile) Close() error {
	if ef.parentArchive != nil {
		if err := ef.parentArchive.Close(); err != nil {
			return err
		}
	}
	if ef.ReadCloser != nil {
		return ef.ReadCloser.Close()
	}
	return nil
}

// If this represents the root of the archive, we use the archive's FileInfo which says it's a file, not a directory.
// The whole point of this package is to treat the archive as a directory, so always return true.
func (dirFile) IsDir() bool {
	return true
}

func (df *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		return df.entries, nil
	}

	if df.entriesRead >= len(df.entries) {
		return nil, io.EOF
	}

	if df.entriesRead+n > len(df.entries) {
		n = len(df.entries) - df.entriesRead
	}

	entries := df.entries[df.entriesRead : df.entriesRead+n]
	df.entriesRead += n

	return entries, nil
}

func (dirFileInfo) Size() int64 {
	return 0
}

func (info dirFileInfo) Mode() fs.FileMode {
	return info.FileInfo.Mode() | fs.ModeDir
}

func (dirFileInfo) IsDir() bool {
	return true
}
