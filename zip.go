package compressor

type Zip struct {
	// Only compress files which are not already in a compressed format.
	SelectiveCompression bool

	// Method or algorithm for compressing stored files.
	Compression uint16

	// If true, errors that occurred while reading or writing a file in the archive
	// will be logged and the operation will continue for the remaining files.
	ContinueOnError bool

	// Encoding for files in zip archives whose names and comments are not UTF-8 encoded.
	TextEncoding string
}
