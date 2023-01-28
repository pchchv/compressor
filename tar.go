package compressor

import (
	"archive/tar"
	"io"
	"strings"
)

type Tar struct {
	// If true, errors that occurred while reading or writing a file in the archive
	// will be logged and the operation will continue for the remaining files.
	ContinueOnError bool
}

func init() {
	RegisterFormat(Tar{})
}

func (Tar) Name() string {
	return ".tar"
}

func (t Tar) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), t.Name()) {
		mr.ByName = true
	}

	// match file header
	r := tar.NewReader(stream)
	_, err := r.Next()
	mr.ByStream = err == nil

	return mr, nil
}
