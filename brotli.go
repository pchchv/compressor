package compressor

import (
	"io"
	"strings"
)

// Brotli facilitates brotli compression.
type Brotli struct {
	Quality int
}

func init() {
	RegisterFormat(Brotli{})
}

func (Brotli) Name() string {
	return ".br"
}

func (br Brotli) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), br.Name()) {
		mr.ByName = true
	}

	// brotli does not have well-defined file headers.
	// The best way to match a stream would be to try to decode part of it,
	// and that has not yet been implemented

	return mr, nil
}
