package compressor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/pchchv/golog"
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

// Extract extracts files from z by implementing the Extractor interface.
// sourceArchive must be io.ReaderAt and io.Seeker, which, oddly enough,
// are mismatched interfaces from io.Reader, which requires a method signature.
// This signature is chosen for the interface because you can Read() from anything you can Read() or Seek().
// Because of the nature of the zip archive format, if sourceArchive is not io.Seeker and io.ReaderAt, an error is returned.
func (z SevenZip) Extract(ctx context.Context, sourceArchive io.Reader, pathsInArchive []string, handleFile FileHandler) error {
	sra, ok := sourceArchive.(seekReaderAt)
	if !ok {
		return fmt.Errorf("input type must be an io.ReaderAt and io.Seeker because of zip format constraints")
	}

	size, err := streamSizeBySeeking(sra)
	if err != nil {
		return fmt.Errorf("determining stream size: %w", err)
	}

	zr, err := sevenzip.NewReaderWithPassword(sra, size, z.Password)
	if err != nil {
		return err
	}

	// important to initialize to non-nil, empty value due to how fileIsIncluded works
	skipDirs := skipList{}

	for i, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return err // honor context cancellation
		}

		if !fileIsIncluded(pathsInArchive, f.Name) {
			continue
		}
		if fileIsIncluded(skipDirs, f.Name) {
			continue
		}

		file := File{
			FileInfo: f.FileInfo(),
			Header:   f.FileHeader,
			FileName: f.Name,
			Open:     func() (io.ReadCloser, error) { return f.Open() },
		}

		err := handleFile(ctx, file)
		if errors.Is(err, fs.SkipDir) {
			// if a directory, skip this path; if a file, skip the folder path
			dirPath := f.Name
			if !file.IsDir() {
				dirPath = path.Dir(f.Name) + "/"
			}
			skipDirs.add(dirPath)
		} else if err != nil {
			if z.ContinueOnError {
				golog.Info("[ERROR] %s: %v", f.Name, err)
				continue
			}
			return fmt.Errorf("handling file %d: %s: %w", i, f.Name, err)
		}
	}

	return nil
}
