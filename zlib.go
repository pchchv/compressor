package compressor

// Zlib facilitates zlib compression.
type Zlib struct {
	CompressionLevel int
}

func init() {
	RegisterFormat(Zlib{})
}
