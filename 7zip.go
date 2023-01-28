package compressor

type SevenZip struct {
	// If true, errors that occurred while reading or writing a file in the archive
	// will be logged and the operation will continue for the remaining files.
	ContinueOnError bool

	// The password, if dealing with an encrypted archive.
	Password string
}

func init() {
	RegisterFormat(SevenZip{})
}
