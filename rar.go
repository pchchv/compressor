package compressor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/nwaples/rardecode/v2"
)

type Rar struct {
	// If true, errors that occurred while reading or writing a file in the archive
	// will be logged and the operation will continue for the remaining files.
	ContinueOnError bool

	// Password to open archives.
	Password string
}

// rarFileInfo satisfies the fs.FileInfo interface for RAR entries.
type rarFileInfo struct {
	fh *rardecode.FileHeader
}

var (
	rarHeaderV1_5 = []byte("Rar!\x1a\x07\x00")     // v1.5
	rarHeaderV5_0 = []byte("Rar!\x1a\x07\x01\x00") // v5.0
)

func init() {
	RegisterFormat(Rar{})
}

func (Rar) Name() string { return ".rar" }

func (r Rar) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), r.Name()) {
		mr.ByName = true
	}

	// match file header (there are two versions; allocate buffer for larger one)
	buf, err := readAtMost(stream, len(rarHeaderV5_0))
	if err != nil {
		return mr, err
	}

	matchedV1_5 := len(buf) >= len(rarHeaderV1_5) &&
		bytes.Equal(rarHeaderV1_5, buf[:len(rarHeaderV1_5)])
	matchedV5_0 := len(buf) >= len(rarHeaderV5_0) &&
		bytes.Equal(rarHeaderV5_0, buf[:len(rarHeaderV5_0)])

	mr.ByStream = matchedV1_5 || matchedV5_0

	return mr, nil
}

// Archive is not implemented for RAR,
// but the method exists so that Rar satisfies the ArchiveFormat interface.
func (r Rar) Archive(_ context.Context, _ io.Writer, _ []File) error {
	return fmt.Errorf("not implemented because RAR is a proprietary format")
}

func (rfi rarFileInfo) Name() string       { return path.Base(rfi.fh.Name) }
func (rfi rarFileInfo) Size() int64        { return rfi.fh.UnPackedSize }
func (rfi rarFileInfo) Mode() os.FileMode  { return rfi.fh.Mode() }
func (rfi rarFileInfo) ModTime() time.Time { return rfi.fh.ModificationTime }
func (rfi rarFileInfo) IsDir() bool        { return rfi.fh.IsDir }
func (rfi rarFileInfo) Sys() interface{}   { return nil }
