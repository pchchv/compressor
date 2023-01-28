package compressor

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"

	"github.com/klauspost/pgzip"
)

// Gz facilitates gzip compression.
type Gz struct {
	// Gzip compression level.
	// If 0, DefaultCompression is assumed, not no compression.
	CompressionLevel int

	// Use a fast parallel Gzip implementation.
	// This is effective only for large threads (about 1 MB or more).
	Multithreaded bool
}

// magic number at the beginning of gzip files
var gzHeader = []byte{0x1f, 0x8b}

func init() {
	RegisterFormat(Gz{})
}

func (Gz) Name() string {
	return ".gz"
}

func (gz Gz) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), gz.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(gzHeader))
	if err != nil {
		return mr, err
	}

	mr.ByStream = bytes.Equal(buf, gzHeader)

	return mr, nil
}

func (gz Gz) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	var wc io.WriteCloser
	var err error

	// The default compression level is 0, not no compression.
	// The lack of compression in the gzipped file makes no sense in this project.
	level := gz.CompressionLevel
	if level == 0 {
		level = gzip.DefaultCompression
	}

	if gz.Multithreaded {
		wc, err = pgzip.NewWriterLevel(w, level)
	} else {
		wc, err = gzip.NewWriterLevel(w, level)
	}

	return wc, err
}
