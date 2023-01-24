package compressor

import (
	"bytes"
	"io"
)

// MatchResult returns true if the format was found either by name, by stream, or by both parameters.
// The name usually refers to searching by file extension,
// and the stream refers to reading the first few bytes of the stream (its header).
// Matching by stream is usually more reliable,
// because filenames do not always indicate the contents of files, if they exist at all.
type MatchResult struct {
	ByName,
	ByStream bool
}

// rewindReader is a reader that can be rewound (reset) to re-read what has already been read
// and then continue reading further from the main stream. When rewind is no longer needed,
// call reader() to get a new reader that first reads the buffered bytes and then continues reading from the stream.
// This is useful for "peeking" into the stream for an arbitrary number of bytes.
type rewindReader struct {
	io.Reader
	buf       *bytes.Buffer
	bufReader io.Reader
}

// Matched returns true if a match was made by either name or stream.
func (mr MatchResult) Matched() bool {
	return mr.ByName || mr.ByStream
}

func newRewindReader(r io.Reader) *rewindReader {
	return &rewindReader{
		Reader: r,
		buf:    new(bytes.Buffer),
	}
}
