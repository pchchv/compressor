package compressor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

type SevenZip struct {
	// If true, errors that occurred while reading or writing a file in the archive
	// will be logged and the operation will continue for the remaining files.
	ContinueOnError bool

	// The password, if dealing with an encrypted archive.
	Password string
}

var sevenZipHeader = []byte("7z\xBC\xAF\x27\x1C")

func init() {
	RegisterFormat(SevenZip{})
}

func (z SevenZip) Name() string {
	return ".7z"
}

func (z SevenZip) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), z.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(sevenZipHeader))
	if err != nil {
		return mr, err
	}

	mr.ByStream = bytes.Equal(buf, sevenZipHeader)

	return mr, nil
}

// Archive is not implemented for 7z,
// but the method exists so that SevenZip satisfies the ArchiveFormat interface.
func (z SevenZip) Archive(_ context.Context, _ io.Writer, _ []File) error {
	return fmt.Errorf("not implemented for 7z because there is no pure Go implementation found")
}
