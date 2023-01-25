package compressor

import (
	"archive/zip"
	"io"

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
