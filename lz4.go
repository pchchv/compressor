package compressor

import (
	"bytes"
	"io"
	"strings"
)

// Lz4 facilitates LZ4 compression.
type Lz4 struct {
	CompressionLevel int
}

var lz4Header = []byte{0x04, 0x22, 0x4d, 0x18}

func init() {
	RegisterFormat(Lz4{})
}

func (Lz4) Name() string {
	return ".lz4"
}

func (lz Lz4) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), lz.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(lz4Header))
	if err != nil {
		return mr, err
	}

	mr.ByStream = bytes.Equal(buf, lz4Header)

	return mr, nil
}
