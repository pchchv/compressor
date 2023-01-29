package compressor

import "github.com/klauspost/compress/zstd"

// Zstd facilitates Zstandard compression.
type Zstd struct {
	EncoderOptions []zstd.EOption
	DecoderOptions []zstd.DOption
}

func init() {
	RegisterFormat(Zstd{})
}
