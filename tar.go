package compressor

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/pchchv/golog"
)

type Tar struct {
	// If true, errors that occurred while reading or writing a file in the archive
	// will be logged and the operation will continue for the remaining files.
	ContinueOnError bool
}

// Interface guards
var (
	_ Archiver  = (*Tar)(nil)
	_ Extractor = (*Tar)(nil)
	_ Inserter  = (*Tar)(nil)
)

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

func (t Tar) Insert(ctx context.Context, into io.ReadWriteSeeker, files []File) error {
	// Tar files may end with some, none, or a lot of zero-byte padding.
	// According to the specification it should end with two 512-byte trailer records consisting solely
	// of null/0 bytes. However, this is not always the case.
	// It looks like the only reliable solution is to scan the entire archive to find the last file,
	// read its size, then use that to calculate the end of contentы and thus the true length of end-of-archive padding.
	// This is a bit more complicated than just adding the size of the last file to the current stream/seek position,
	// because we have to accurately align the 512-byte blocks.
	// Another option is to scan the file for the last continuous series 0, without interpreting the tar format at all,
	// and find the nearest block size offset and start writing there.
	// The problem is that you won't know if you've overwritten part of the last file if it ends with all 0s.
	var lastFileSize, lastStreamPos int64
	const blockSize = 512 // (as of Go 1.17, this is also a hard-coded const in the archive/tar package)
	tr := tar.NewReader(into)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		lastStreamPos, err = into.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}

		lastFileSize = hdr.Size
	}

	// now calculate the exact location for writing the new file
	newOffset := lastStreamPos + lastFileSize
	newOffset += blockSize - (newOffset % blockSize) // shift to next-nearest block boundary
	_, err := into.Seek(newOffset, io.SeekStart)
	if err != nil {
		return err
	}

	tw := tar.NewWriter(into)
	defer tw.Close()

	for i, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}

		err = t.writeFileToArchive(ctx, tw, file)
		if err != nil {
			if t.ContinueOnError && ctx.Err() == nil {
				golog.Info("[ERROR] appending file %d into archive: %s: %v", i, file.Name(), err)
				continue
			}
			return fmt.Errorf("appending file %d into archive: %s: %w", i, file.Name(), err)
		}
	}

	return nil
}

func (t Tar) Extract(ctx context.Context, sourceArchive io.Reader, pathsInArchive []string, handleFile FileHandler) error {
	tr := tar.NewReader(sourceArchive)
	// important to initialize to non-nil, empty value due to how fileIsIncluded works
	skipDirs := skipList{}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if t.ContinueOnError && ctx.Err() == nil {
				golog.Info("[ERROR] Advancing to next file in tar archive: %v", err)
				continue
			}
			return err
		}

		if !fileIsIncluded(pathsInArchive, hdr.Name) {
			continue
		}
		if fileIsIncluded(skipDirs, hdr.Name) {
			continue
		}

		if hdr.Typeflag == tar.TypeXGlobalHeader {
			// ignore the pax global header from git-generated tarballs
			continue
		}

		file := File{
			FileInfo:   hdr.FileInfo(),
			Header:     hdr,
			FileName:   hdr.Name,
			LinkTarget: hdr.Linkname,
			Open:       func() (io.ReadCloser, error) { return io.NopCloser(tr), nil },
		}

		err = handleFile(ctx, file)
		if errors.Is(err, fs.SkipDir) {
			// if a directory, skip this path
			// if a file, skip the folder path
			dirPath := hdr.Name

			if hdr.Typeflag != tar.TypeDir {
				dirPath = path.Dir(hdr.Name) + "/"
			}

			skipDirs.add(dirPath)
		} else if err != nil {
			return fmt.Errorf("handling file: %s: %w", hdr.Name, err)
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
