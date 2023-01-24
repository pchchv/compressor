package compressor

import "io"

// The format is either an archive or a compression format.
type Format interface {
	// Name returns the name of the format.
	Name() string

	// Match returns true if the given name/stream is recognized. One of the arguments is optional:
	// the filename can be empty if you are working with an unnamed stream,
	// or the stream can be empty if you are working with just the filename.
	// The filename should consist only of the filename, not the path component,
	// and is usually used to search by file extension.
	// However, it is preferable to perform a read stream search.
	// Match reads only as many bytes as necessary to determine the match.
	// To save the stream when matching,
	// you must either buffer what Match reads or search for the last position before calling Match.
	Match(filename string, stream io.Reader) (MatchResult, error)
}

// Compressor can compress data by wrapping a writer.
type Compressor interface {
	// OpenWriter wraps w with a new writer that compresses what is written.
	// The writer must be closed when writing is finished.
	OpenWriter(w io.Writer) (io.WriteCloser, error)
}

// Decompressor can decompress data by wrapping a reader.
type Decompressor interface {
	// OpenReader wraps r with a new reader that decompresses what is read.
	// The reader must be closed when reading is finished.
	OpenReader(r io.Reader) (io.ReadCloser, error)
}

// Compression is a compression format with both compress and decompress methods.
type Compression interface {
	Format
	Compressor
	Decompressor
}
