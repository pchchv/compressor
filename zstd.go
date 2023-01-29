package compressor

import (
	"bytes"
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// Zstd facilitates Zstandard compression.
type Zstd struct {
	EncoderOptions []zstd.EOption
	DecoderOptions []zstd.DOption
}

type errorCloser struct {
	*zstd.Decoder
}

// magic number at the beginning of Zstandard files
var zstdHeader = []byte{0x28, 0xb5, 0x2f, 0xfd}

func init() {
	RegisterFormat(Zstd{})
}

func (Zstd) Name() string {
	return ".zst"
}

func (zs Zstd) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), zs.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(zstdHeader))
	if err != nil {
		return mr, err
	}

	mr.ByStream = bytes.Equal(buf, zstdHeader)

	return mr, nil
}

func (zs Zstd) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	return zstd.NewWriter(w, zs.EncoderOptions...)
}

func (zs Zstd) OpenReader(r io.Reader) (io.ReadCloser, error) {
	zr, err := zstd.NewReader(r, zs.DecoderOptions...)
	if err != nil {
		return nil, err
	}

	return errorCloser{zr}, nil
}

func (ec errorCloser) Close() error {
	ec.Decoder.Close()
	return nil
}
