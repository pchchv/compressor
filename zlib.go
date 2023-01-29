package compressor

import (
	"bytes"
	"compress/zlib"
	"io"
	"strings"
)

// Zlib facilitates zlib compression.
type Zlib struct {
	CompressionLevel int
}

var ZlibHeader = []byte{0x78}

func init() {
	RegisterFormat(Zlib{})
}

func (Zlib) Name() string {
	return ".zz"
}

func (zz Zlib) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), zz.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(ZlibHeader))
	if err != nil {
		return mr, err
	}

	mr.ByStream = bytes.Equal(buf, ZlibHeader)

	return mr, nil
}

func (zz Zlib) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	level := zz.CompressionLevel

	if level == 0 {
		level = zlib.DefaultCompression
	}

	return zlib.NewWriterLevel(w, level)
}

func (Zlib) OpenReader(r io.Reader) (io.ReadCloser, error) {
	return zlib.NewReader(r)
}
