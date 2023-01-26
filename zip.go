package compressor

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/dsnet/compress/bzip2"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

type Zip struct {
	// Only compress files which are not already in a compressed format.
	SelectiveCompression bool

	// Method or algorithm for compressing stored files.
	Compression uint16

	// If true, errors that occurred while reading or writing a file in the archive
	// will be logged and the operation will continue for the remaining files.
	ContinueOnError bool

	// Encoding for files in zip archives whose names and comments are not UTF-8 encoded.
	TextEncoding string
}

const (
	// Additional compression methods not offered by archive/zip.
	ZipMethodBzip2 = 12
	ZipMethodLzma  = 14
	ZipMethodZstd  = 93
	ZipMethodXz    = 95
)

var (
	// headers of empty zip files might end with 0x05,0x06 or 0x06,0x06 instead of 0x03,0x04
	zipHeader = []byte("PK\x03\x04")

	// compressedFormats is an incomplete set of file extensions with lowercase letters
	// for formats that are normally already compressed.
	// Compressing already compressed files is inefficient.
	compressedFormats = map[string]struct{}{
		".7z":   {},
		".avi":  {},
		".br":   {},
		".bz2":  {},
		".cab":  {},
		".docx": {},
		".gif":  {},
		".gz":   {},
		".jar":  {},
		".jpeg": {},
		".jpg":  {},
		".lz":   {},
		".lz4":  {},
		".lzma": {},
		".m4v":  {},
		".mov":  {},
		".mp3":  {},
		".mp4":  {},
		".mpeg": {},
		".mpg":  {},
		".png":  {},
		".pptx": {},
		".rar":  {},
		".sz":   {},
		".tbz2": {},
		".tgz":  {},
		".tsz":  {},
		".txz":  {},
		".xlsx": {},
		".xz":   {},
		".zip":  {},
		".zipx": {},
	}
)

func init() {
	RegisterFormat(Zip{}) // Not implement! In progress...
	zip.RegisterCompressor(ZipMethodBzip2, func(out io.Writer) (io.WriteCloser, error) {
		return bzip2.NewWriter(out, &bzip2.WriterConfig{ /*TODO: Level: z.CompressionLevel*/ })
	})

	zip.RegisterCompressor(ZipMethodZstd, func(out io.Writer) (io.WriteCloser, error) {
		return zstd.NewWriter(out)
	})

	zip.RegisterCompressor(ZipMethodXz, func(out io.Writer) (io.WriteCloser, error) {
		return xz.NewWriter(out)
	})

	zip.RegisterDecompressor(ZipMethodBzip2, func(r io.Reader) io.ReadCloser {
		bz2r, err := bzip2.NewReader(r, nil)
		if err != nil {
			return nil
		}
		return bz2r
	})

	zip.RegisterDecompressor(ZipMethodZstd, func(r io.Reader) io.ReadCloser {
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil
		}
		return zr.IOReadCloser()
	})

	zip.RegisterDecompressor(ZipMethodXz, func(r io.Reader) io.ReadCloser {
		xr, err := xz.NewReader(r)
		if err != nil {
			return nil
		}
		return io.NopCloser(xr)
	})
}

func (z Zip) Name() string {
	return ".zip"
}

func (z Zip) Match(filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), z.Name()) {
		mr.ByName = true
	}

	// match file header
	buf, err := readAtMost(stream, len(zipHeader))
	if err != nil {
		return mr, err
	}

	mr.ByStream = bytes.Equal(buf, zipHeader)

	return mr, nil
}

func (z Zip) archiveOneFile(ctx context.Context, zw *zip.Writer, idx int, file File) error {
	if err := ctx.Err(); err != nil {
		return err // honor context cancellation
	}

	hdr, err := zip.FileInfoHeader(file)
	if err != nil {
		return fmt.Errorf("getting info for file %d: %s: %w", idx, file.Name(), err)
	}
	hdr.Name = file.FileName // complete path, since FileInfoHeader() only has base name

	// customize header based on file properties
	if file.IsDir() {
		if !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/" // required
		}
		hdr.Method = zip.Store
	} else if z.SelectiveCompression {
		// only enable compression on compressable files
		ext := strings.ToLower(path.Ext(hdr.Name))
		if _, ok := compressedFormats[ext]; ok {
			hdr.Method = zip.Store
		} else {
			hdr.Method = z.Compression
		}
	}

	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return fmt.Errorf("creating header for file %d: %s: %w", idx, file.Name(), err)
	}

	// directories have no file body
	if file.IsDir() {
		return nil
	}
	if err := openAndCopyFile(file, w); err != nil {
		return fmt.Errorf("writing file %d: %s: %w", idx, file.Name(), err)
	}

	return nil
}
