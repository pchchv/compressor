package compressor

// Gz facilitates gzip compression.
type Gz struct {
	// Gzip compression level.
	// If 0, DefaultCompression is assumed, not no compression.
	CompressionLevel int

	// Use a fast parallel Gzip implementation.
	// This is effective only for large threads (about 1 MB or more).
	Multithreaded bool
}

func init() {
	RegisterFormat(Gz{})
}
