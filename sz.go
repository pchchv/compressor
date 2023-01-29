package compressor

// Sz facilitates Snappy compression.
type Sz struct{}

func init() {
	RegisterFormat(Sz{})
}
