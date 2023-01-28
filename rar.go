package compressor

type Rar struct {
	// If true, errors that occurred while reading or writing a file in the archive
	// will be logged and the operation will continue for the remaining files.
	ContinueOnError bool

	// Password to open archives.
	Password string
}

func init() {
	RegisterFormat(Rar{})
}
