package compressor

import (
	"bytes"
	"io"
	"strings"
)

// Bz2 facilitates bzip2 compression.
type Bz2 struct {
	CompressionLevel int
}

var bzip2Header = []byte("BZh")

func init() {
	RegisterFormat(Bz2{})
}

func (Bz2) Name() string {
	return ".bz2"
}

func (bz Bz2) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), bz.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(bzip2Header))
	if err != nil {
		return mr, err
	}

	mr.ByStream = bytes.Equal(buf, bzip2Header)

	return mr, nil
}
