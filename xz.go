package compressor

// Xz facilitates xz compression.
type Xz struct{}

// magic number at the beginning of xz files.
var xzHeader = []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}

func init() {
	RegisterFormat(Xz{})
}
