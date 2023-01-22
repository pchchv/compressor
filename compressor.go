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
