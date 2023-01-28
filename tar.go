package compressor

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pchchv/golog"
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

func (t Tar) Archive(ctx context.Context, output io.Writer, files []File) error {
	tw := tar.NewWriter(output)
	defer tw.Close()

	for _, file := range files {
		if err := t.writeFileToArchive(ctx, tw, file); err != nil {
			if t.ContinueOnError && ctx.Err() == nil { // context errors should always abort
				golog.Info("[ERROR] %v", err)
				continue
			}
			return err
		}
	}

	return nil
}

func (t Tar) ArchiveAsync(ctx context.Context, output io.Writer, files <-chan File) error {
	tw := tar.NewWriter(output)
	defer tw.Close()

	for file := range files {
		if err := t.writeFileToArchive(ctx, tw, file); err != nil {
			if t.ContinueOnError && ctx.Err() == nil { // context errors should always abort
				golog.Info("[ERROR] %v", err)
				continue
			}
			return err
		}
	}

	return nil
}

func (Tar) writeFileToArchive(ctx context.Context, tw *tar.Writer, file File) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	hdr, err := tar.FileInfoHeader(file, file.LinkTarget)
	if err != nil {
		return fmt.Errorf("file %s: creating header: %w", file.FileName, err)
	}

	hdr.Name = file.FileName // complete path, since FileInfoHeader() only has base name

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("file %s: writing header: %w", file.FileName, err)
	}

	// write the file body only if it actually exists
	// (directories and links do not have a body)
	if hdr.Typeflag != tar.TypeReg {
		return nil
	}

	if err := openAndCopyFile(file, tw); err != nil {
		return fmt.Errorf("file %s: writing data: %w", file.FileName, err)
	}

	return nil
}
