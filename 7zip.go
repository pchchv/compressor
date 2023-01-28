package compressor

import (
	"context"
	"fmt"
	"io"
)

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

func (z SevenZip) Name() string {
	return ".7z"
}

// Archive is not implemented for 7z,
// but the method exists so that SevenZip satisfies the ArchiveFormat interface.
func (z SevenZip) Archive(_ context.Context, _ io.Writer, _ []File) error {
	return fmt.Errorf("not implemented for 7z because there is no pure Go implementation found")
}
