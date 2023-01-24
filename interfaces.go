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
