package compressor

import (
	"io"
	"io/fs"
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
	FollowSymolicLinks bool

	// If true, some attributes of the file will not be saved.
	// The name, size, type and permissions will be saved.
	ClearAttributes bool
}
