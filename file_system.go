package compressor

import (
	"context"
	"errors"
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

// compressedFile is an fs.File that specially reads from a decompression reader,
// and which closes both that reader and the underlying file.
type compressedFile struct {
	*os.File
	decomp io.ReadCloser
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

// context always returns context, preferring f.Context if not nil.
func (f ArchiveFS) context() context.Context {
	if f.Context != nil {
		return f.Context
	}

	return context.Background()
}

// Open opens the named file from the archive. If name is ".",
// the archive file itself will be opened as a directory file.
func (f ArchiveFS) Open(name string) (archiveFile fs.File, err error) {
	var files []File
	var found bool
	var inputStream io.Reader = archiveFile

	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	if f.Path != "" {
		archiveFile, err = os.Open(f.Path)
		if err != nil {
			return nil, err
		}
		defer func() {
			// close the archive file if the extraction fails
			if err != nil {
				archiveFile.Close()
			}
		}()
	} else if f.Stream != nil {
		archiveFile = fakeArchiveFile{}
	}

	// apply prefix if fs is rooted in a subtree
	name = path.Join(f.Prefix, name)

	// handle special case of opening the archive root
	if name == "." && archiveFile != nil {
		archiveInfo, err := archiveFile.Stat()
		if err != nil {
			return nil, err
		}

		entries, err := f.ReadDir(name)
		if err != nil {
			return nil, err
		}

		return &dirFile{
			extractedFile: extractedFile{
				File: File{
					FileInfo: dirFileInfo{archiveInfo},
					FileName: ".",
				},
			},
			entries: entries,
		}, nil
	}

	// collect them all or stop at exact file match, note we don't stop at folder match
	handler := func(_ context.Context, file File) error {
		file.FileName = strings.Trim(file.FileName, "/")
		files = append(files, file)
		if file.FileName == name && !file.IsDir() {
			found = true
			return errors.New("stop walk")
		}
		return nil
	}

	if f.Stream != nil {
		inputStream = io.NewSectionReader(f.Stream, 0, f.Stream.Size())
	}

	err = f.Format.Extract(f.context(), inputStream, []string{name}, handler)
	if found {
		err = nil
	}
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fs.ErrNotExist
	}

	// exactly one or exact file found, test name match to detect implicit dir name https://github.com/mholt/archiver/issues/340
	if (len(files) == 1 && files[0].FileName == name) || found {
		file := files[len(files)-1]
		if file.IsDir() {
			return &dirFile{extractedFile: extractedFile{File: file}}, nil
		}

		// if named file is not a regular file, it can't be opened
		if !file.Mode().IsRegular() {
			return extractedFile{File: file}, nil
		}

		// regular files can be read, so open it for reading
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		return extractedFile{File: file, ReadCloser: rc, parentArchive: archiveFile}, nil
	}

	// implicit files
	files = fillImplicit(files)
	file := search(name, files)
	if file == nil {
		return nil, fs.ErrNotExist
	}

	if file.IsDir() {
		return &dirFile{extractedFile: extractedFile{File: *file}, entries: openReadDir(name, files)}, nil
	}

	// if named file is not a regular file, it can't be opened
	if !file.Mode().IsRegular() {
		return extractedFile{File: *file}, nil
	}

	// regular files can be read, so open it for reading
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}

	return extractedFile{File: *file, ReadCloser: rc, parentArchive: archiveFile}, nil
}

// ReadDir reads the named directory from within the archive.
func (f ArchiveFS) ReadDir(name string) ([]fs.DirEntry, error) {
	var inputStream io.Reader
	var archiveFile *os.File
	var filter []string
	var foundFile bool
	var files []File
	var err error

	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}

	if f.Stream == nil {
		archiveFile, err = os.Open(f.Path)
		if err != nil {
			return nil, err
		}
		defer archiveFile.Close()
	}

	// apply prefix if fs is rooted in a subtree
	name = path.Join(f.Prefix, name)

	handler := func(_ context.Context, file File) error {
		file.FileName = strings.Trim(file.FileName, "/")
		files = append(files, file)
		if file.FileName == name && !file.IsDir() {
			foundFile = true
			return errors.New("stop walk")
		}
		return nil
	}

	// handle special case of reading from root of archive
	if name != "." {
		filter = []string{name}
	}

	inputStream = archiveFile
	if f.Stream != nil {
		inputStream = io.NewSectionReader(f.Stream, 0, f.Stream.Size())
	}

	err = f.Format.Extract(f.context(), inputStream, filter, handler)
	if foundFile {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: errors.New("not a dir")}
	}
	if err != nil {
		return nil, err
	}

	// always find all implicit directories
	files = fillImplicit(files)
	// and return early for dot file
	if name == "." {
		return openReadDir(name, files), nil
	}

	file := search(name, files)
	if file == nil {
		return nil, fs.ErrNotExist
	}

	if !file.IsDir() {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: errors.New("not a dir")}
	}

	return openReadDir(name, files), nil
}

func (cf compressedFile) Read(p []byte) (int, error) {
	return cf.decomp.Read(p)
}

func (cf compressedFile) Close() (err error) {
	err = cf.File.Close()
	if err == nil {
		return cf.decomp.Close()
	}

	_ = cf.decomp.Close()

	return
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
