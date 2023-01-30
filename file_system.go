package compressor

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
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

// ArchiveFS allows accessing an archive (or a compressed archive) using a consistent file system interface.
// Essentially, it allows traversal and read the contents of an archive just like any normal directory on disk.
// The contents of compressed archives are transparently decompressed.
// A valid ArchiveFS value should be set either Path or Stream.
// If Path is set, a literal file will be opened from the disk.
// If Stream is set, new SectionReaders will be implicitly created to access the stream, providing safe concurrent access.
//
// Because of the Go file system APIs (see io/fs package), tArchiveFS performance when using fs.WalkDir()
// is low for archives with lots of files.
// The fs.WalkDir() API requires listing the contents of each directory in turn,
// and the only way to ensure we return a complete list of folder contents is to traverse the whole archive and build a slice,
// so if this is done for the root of an archive with many files,
// performance tends to O(n^2) as the entire archive is walked for each folder that is enumerated (WalkDir calls ReadDir recursively).
// If you don't want the contents of each directory to be viewed in order, prefer to call Extract() from the archive type directly,
// this will do an O(n) view of the contents in archive order, rather than the slower directory tree order.
type ArchiveFS struct {
	// set one of these:
	Path   string            // path to the archive file on disk
	Stream *io.SectionReader // stream from which to read archive

	Format  Archival        // the archive format
	Prefix  string          // optional subdirectory in which to root the fs
	Context context.Context // optional
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

func split(name string) (dir, elem string, isDir bool) {
	if name[len(name)-1] == '/' {
		isDir = true
		name = name[:len(name)-1]
	}

	i := len(name) - 1
	for i >= 0 && name[i] != '/' {
		i--
	}

	if i < 0 {
		return ".", name, isDir
	}

	return name[:i], name[i+1:], isDir
}

// Modified from zip.Reader initFileList, it's used to find all implicit dirs.
func fillImplicit(files []File) []File {
	dirs := make(map[string]bool)
	knownDirs := make(map[string]bool)
	entries := make([]File, 0, 0)

	for _, file := range files {
		for dir := path.Dir(file.FileName); dir != "."; dir = path.Dir(dir) {
			dirs[dir] = true
		}
		entries = append(entries, file)
		if file.IsDir() {
			knownDirs[file.FileName] = true
		}
	}
	for dir := range dirs {
		if !knownDirs[dir] {
			entries = append(entries, File{FileInfo: implicitDirInfo{implicitDirEntry{path.Base(dir)}}, FileName: dir})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		fi, fj := entries[i], entries[j]
		di, ei, _ := split(fi.FileName)
		dj, ej, _ := split(fj.FileName)

		if di != dj {
			return di < dj
		}
		return ei < ej
	})
	return entries
}

// modified from zip.Reader openLookup
func search(name string, entries []File) *File {
	dir, elem, _ := split(name)
	i := sort.Search(len(entries), func(i int) bool {
		idir, ielem, _ := split(entries[i].FileName)
		return idir > dir || idir == dir && ielem >= elem
	})

	if i < len(entries) {
		fname := entries[i].FileName
		if fname == name || len(fname) == len(name)+1 && fname[len(name)] == '/' && fname[:len(name)] == name {
			return &entries[i]
		}
	}

	return nil
}

// modified from zip.Reader openReadDir
func openReadDir(dir string, entries []File) []fs.DirEntry {
	i := sort.Search(len(entries), func(i int) bool {
		idir, _, _ := split(entries[i].FileName)
		return idir >= dir
	})
	j := sort.Search(len(entries), func(j int) bool {
		jdir, _, _ := split(entries[j].FileName)
		return jdir > dir
	})
	dirs := make([]fs.DirEntry, j-i)

	for idx := range dirs {
		dirs[idx] = fs.FileInfoToDirEntry(entries[i+idx])
	}

	return dirs
}
