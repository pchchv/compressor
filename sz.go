package compressor

import (
	"bytes"
	"io"
	"strings"
)

// Sz facilitates Snappy compression.
type Sz struct{}

var snappyHeader = []byte{0xff, 0x06, 0x00, 0x00, 0x73, 0x4e, 0x61, 0x50, 0x70, 0x59}

func init() {
	RegisterFormat(Sz{})
}

func (sz Sz) Name() string {
	return ".sz"
}

func (sz Sz) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), sz.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(snappyHeader))
	if err != nil {
		return mr, err
	}

	mr.ByStream = bytes.Equal(buf, snappyHeader)

	return mr, nil
}
