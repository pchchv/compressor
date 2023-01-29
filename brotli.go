package compressor

// Brotli facilitates brotli compression.
type Brotli struct {
	Quality int
}

func init() {
	RegisterFormat(Brotli{})
}
