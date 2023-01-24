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

func (rr *rewindReader) Read(p []byte) (n int, err error) {
	// If there is a buffer from which we have to read, we start with it.
	// Read from the main stream only after the buffer is "depleted"
	if rr.bufReader != nil {
		n, err = rr.bufReader.Read(p)

		if err == io.EOF {
			rr.bufReader = nil
			err = nil
		}

		if n == len(p) {
			return
		}
	}

	// buffer has been "depleted" so read from underlying connection
	nr, err := rr.Reader.Read(p[n:])

	// everything that was read should be written to the buffer, even if there was an error
	if nr > 0 {
		if nw, errw := rr.buf.Write(p[n : n+nr]); errw != nil {
			return nw, errw
		}
	}

	// until now n was the number of bytes read from the buffer, and nr was the number of bytes read from the stream.
	// Add them up to get the total number of bytes.
	n += nr

	return
}

// rewind returns the thread to the beginning, forcing Read() to start reading from the beginning of the buffered bytes.
func (rr *rewindReader) rewind() {
	rr.bufReader = bytes.NewReader(rr.buf.Bytes())
}

// reader returns a reader that reads first from the buffered bytes and then from the base stream.
// After this function is called, no more rewinding is allowed,
// since no read from the stream is written, so rewinding is not possible.
// If the base reader implements io.Seeker, the base reader itself will be used.
func (rr *rewindReader) reader() io.Reader {
	if ras, ok := rr.Reader.(io.Seeker); ok {
		if _, err := ras.Seek(0, io.SeekStart); err == nil {
			return rr.Reader
		}
	}
	return io.MultiReader(bytes.NewReader(rr.buf.Bytes()), rr.Reader)
}
