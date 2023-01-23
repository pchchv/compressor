package compressor

import (
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// File abstraction for interacting with archives.
type File struct {
	fs.FileInfo

	// The file header as used/provided by the archive format.
	Header interface{}

	// The path of the file as it appears in the archive.
	Name string

	// For symbolic and hard links.
	// Not all archive formats are supported.
	LinkTarget string

	// A callback function that opens a file to read its contents.
	// The file must be closed when the reading is finished.
	// Not used for files that have no content (directories and links).
	Open func() (io.ReadCloser, error)
}

// FromDiskOptions specifies options for gathering files from the disk.
type FromDiskOptions struct {
	// If true, symbolic links will be dereferenced,
	// that is, the link will not be added as a link,
	// but what the link points to will be added as a file.
	FollowSymboliclinks bool

	// If true, some attributes of the file will not be saved.
	// The name, size, type and permissions will be saved.
	ClearAttributes bool
}

// trimTopDir removes the top or first directory from the path.
// It expects a path with a forward slash.
// For example, "a/b/c" => "b/c".
func trimTopDir(dir string) string {
	if len(dir) > 0 && dir[0] == '/' {
		dir = dir[1:]
	}

	if pos := strings.Index(dir, "/"); pos >= 0 {
		return dir[pos+1:]
	}

	return dir
}

// nameOnDiskToNameInArchive converts the file name from disk to the name in the archive,
// following the rules defined by FilesFromDisk. nameOnDisk is the full name of the file on disk,
// which is expected to be prefixed by rootOnDisk and will be placed in the rootInArchive folder in the archive.
func nameOnDiskToNameInArchive(nameOnDisk, rootOnDisk, rootInArchive string) string {
	if strings.HasSuffix(rootOnDisk, string(filepath.Separator)) {
		rootInArchive = trimTopDir(rootInArchive)
	} else if rootInArchive == "" {
		rootInArchive = filepath.Base(rootOnDisk)
	}

	if strings.HasSuffix(rootInArchive, "/") {
		rootInArchive += filepath.Base(rootOnDisk)
	}

	truncPath := strings.TrimPrefix(nameOnDisk, rootOnDisk)

	return path.Join(rootInArchive, filepath.ToSlash(truncPath))
}
