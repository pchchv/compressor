package compressor

// Bz2 facilitates bzip2 compression.
type Bz2 struct {
	CompressionLevel int
}

func init() {
	RegisterFormat(Bz2{})
}

func (Bz2) Name() string {
	return ".bz2"
}
