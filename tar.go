package compressor

type Tar struct {
	// If true, errors that occurred while reading or writing a file in the archive
	// will be logged and the operation will continue for the remaining files.
	ContinueOnError bool
}

func init() {
	RegisterFormat(Tar{})
}
