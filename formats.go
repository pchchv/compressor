package compressor

import (
	"bytes"
	"context"
	"errors"
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

// CompressedArchive combines a compression format on top of an archive format (e.g. "tar.gz")
// and provides both functionalities in a single type.
// This ensures that archive functions are wrapped by compressors and decompressors.
// However, compressed archives have some limitations.
// For example, files cannot be inserted/appended because of complexities with
// modifying existing compression state.
// As this type is intended to compose compression and archive formats,
// both must be specified for the value to be valid, or its methods will return errors.
type CompressedArchive struct {
	Compression
	Archival
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

// Name returns a concatenation of the archive format name and the compression format name.
func (caf CompressedArchive) Name() string {
	var name string

	if caf.Compression == nil && caf.Archival == nil {
		panic("missing both compression and archive formats")
	}

	if caf.Archival != nil {
		name += caf.Archival.Name()
	}

	if caf.Compression != nil {
		name += caf.Compression.Name()
	}

	return name
}

// Match matches if the input matches both the compression and archive format.
func (caf CompressedArchive) Match(filename string, stream io.Reader) (MatchResult, error) {
	var conglomerate MatchResult

	if caf.Compression != nil {
		matchResult, err := caf.Compression.Match(filename, stream)
		if err != nil {
			return MatchResult{}, err
		}

		if !matchResult.Matched() {
			return matchResult, nil
		}

		// wrap the reader with a decompressor, to match the archive, when reading the stream
		rc, err := caf.Compression.OpenReader(stream)
		if err != nil {
			return matchResult, err
		}

		defer rc.Close()
		stream = rc

		conglomerate = matchResult
	}

	if caf.Archival != nil {
		matchResult, err := caf.Archival.Match(filename, stream)
		if err != nil {
			return MatchResult{}, err
		}

		if !matchResult.Matched() {
			return matchResult, nil
		}

		conglomerate.ByName = conglomerate.ByName || matchResult.ByName
		conglomerate.ByStream = conglomerate.ByStream || matchResult.ByStream
	}

	return conglomerate, nil
}

// Archive adds files to the output archive while compressing the result.
func (caf CompressedArchive) Archive(ctx context.Context, output io.Writer, files []File) error {
	if caf.Compression != nil {
		wc, err := caf.Compression.OpenWriter(output)
		if err != nil {
			return err
		}

		defer wc.Close()
		output = wc
	}
	return caf.Archival.Archive(ctx, output, files)
}

// Extract reads files out of an archive while decompressing the results.
func (caf CompressedArchive) Extract(ctx context.Context, sourceArchive io.Reader, pathsInArchive []string, handleFile FileHandler) error {
	if caf.Compression != nil {
		rc, err := caf.Compression.OpenReader(sourceArchive)
		if err != nil {
			return err
		}

		defer rc.Close()
		sourceArchive = rc
	}
	return caf.Archival.(Extractor).Extract(ctx, sourceArchive, pathsInArchive, handleFile)
}

func identifyOne(format Format, filename string, stream *rewindReader, comp Compression) (mr MatchResult, err error) {
	defer stream.rewind()

	// if the search is in a compressed format, wrap the stream in a reader
	// that can decompress it to match the "internal" format
	// (create a new reader every time we do a match, because we reset/search the stream every time,
	// and it can mess up the state of the compression reader if we don't discard it too)
	if comp != nil {
		decompressedStream, openErr := comp.OpenReader(stream)
		if openErr != nil {
			return MatchResult{}, openErr
		}
		defer decompressedStream.Close()
		mr, err = format.Match(filename, decompressedStream)
	} else {
		mr, err = format.Match(filename, stream)
	}

	// if the error is EOF - ignore it.
	// This means that the input file is small.
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return mr, err
}
