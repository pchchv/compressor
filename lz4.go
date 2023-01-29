package compressor

// Lz4 facilitates LZ4 compression.
type Lz4 struct {
	CompressionLevel int
}

func init() {
	RegisterFormat(Lz4{})
}
