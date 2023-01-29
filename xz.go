package compressor

import (
	"bytes"
	"io"
	"strings"

	"github.com/ulikunitz/xz"
	xxz "github.com/xi2/xz"
)

// Xz facilitates xz compression.
type Xz struct{}

// magic number at the beginning of xz files.
var xzHeader = []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}

func init() {
	RegisterFormat(Xz{})
}

func (Xz) Name() string {
	return ".xz"
}

func (x Xz) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), x.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(xzHeader))
	if err != nil {
		return mr, err
	}

	mr.ByStream = bytes.Equal(buf, xzHeader)

	return mr, nil
}

func (Xz) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	return xz.NewWriter(w)
}

func (Xz) OpenReader(r io.Reader) (io.ReadCloser, error) {
	xr, err := xxz.NewReader(r, 0)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(xr), err
}
