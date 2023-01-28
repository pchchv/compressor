package compressor

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path"
	"strings"

	"github.com/dsnet/compress/bzip2"
	"github.com/klauspost/compress/zstd"
	"github.com/pchchv/golog"
	"github.com/ulikunitz/xz"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
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

type seekReaderAt interface {
	io.ReaderAt
	io.Seeker
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

	encodings = map[string]encoding.Encoding{
		"ibm866":            charmap.CodePage866,
		"iso8859_2":         charmap.ISO8859_2,
		"iso8859_3":         charmap.ISO8859_3,
		"iso8859_4":         charmap.ISO8859_4,
		"iso8859_5":         charmap.ISO8859_5,
		"iso8859_6":         charmap.ISO8859_6,
		"iso8859_7":         charmap.ISO8859_7,
		"iso8859_8":         charmap.ISO8859_8,
		"iso8859_8I":        charmap.ISO8859_8I,
		"iso8859_10":        charmap.ISO8859_10,
		"iso8859_13":        charmap.ISO8859_13,
		"iso8859_14":        charmap.ISO8859_14,
		"iso8859_15":        charmap.ISO8859_15,
		"iso8859_16":        charmap.ISO8859_16,
		"koi8r":             charmap.KOI8R,
		"koi8u":             charmap.KOI8U,
		"macintosh":         charmap.Macintosh,
		"windows874":        charmap.Windows874,
		"windows1250":       charmap.Windows1250,
		"windows1251":       charmap.Windows1251,
		"windows1252":       charmap.Windows1252,
		"windows1253":       charmap.Windows1253,
		"windows1254":       charmap.Windows1254,
		"windows1255":       charmap.Windows1255,
		"windows1256":       charmap.Windows1256,
		"windows1257":       charmap.Windows1257,
		"windows1258":       charmap.Windows1258,
		"macintoshcyrillic": charmap.MacintoshCyrillic,
		"gbk":               simplifiedchinese.GBK,
		"gb18030":           simplifiedchinese.GB18030,
		"big5":              traditionalchinese.Big5,
		"eucjp":             japanese.EUCJP,
		"iso2022jp":         japanese.ISO2022JP,
		"shiftjis":          japanese.ShiftJIS,
		"euckr":             korean.EUCKR,
		"utf16be":           unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
		"utf16le":           unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM),
	}
)

func init() {
	RegisterFormat(Zip{})
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

func (z Zip) Archive(ctx context.Context, output io.Writer, files []File) error {
	zw := zip.NewWriter(output)
	defer zw.Close()

	for i, file := range files {
		if err := z.archiveOneFile(ctx, zw, i, file); err != nil {
			return err
		}
	}

	return nil
}

func (z Zip) ArchiveAsync(ctx context.Context, output io.Writer, files <-chan File) error {
	var i int

	zw := zip.NewWriter(output)
	defer zw.Close()

	for file := range files {
		if err := z.archiveOneFile(ctx, zw, i, file); err != nil {
			if z.ContinueOnError && ctx.Err() == nil { // context errors should always abort
				golog.Error("[ERROR] %v", err)
				continue
			}
			return err
		}
		i++
	}

	return nil
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

// Extract extracts files from z by implementing the Extractor interface.
// sourceArchive must be io.ReaderAt and io.Seeker, which, oddly enough,
// are mismatched interfaces from io.Reader, which requires a method signature.
// This signature is chosen for the interface because you can Read() from anything you can Read() or Seek().
// Because of the nature of the zip archive format, if sourceArchive is not io.Seeker and io.ReaderAt, an error is returned.
func (z Zip) Extract(ctx context.Context, sourceArchive io.Reader, pathsInArchive []string, handleFile FileHandler) error {
	sra, ok := sourceArchive.(seekReaderAt)
	if !ok {
		return fmt.Errorf("input type must be an io.ReaderAt and io.Seeker because of zip format constraints")
	}

	size, err := streamSizeBySeeking(sra)
	if err != nil {
		return fmt.Errorf("determining stream size: %w", err)
	}

	zr, err := zip.NewReader(sra, size)
	if err != nil {
		return err
	}

	// important to initialize to non-nil, empty value due to how fileIsIncluded works
	skipDirs := skipList{}

	for i, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return err // honor context cancellation
		}

		// ensure filename and comment are UTF-8 encoded (issue #147 and PR #305)
		z.decodeText(&f.FileHeader)

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
				log.Printf("[ERROR] %s: %v", f.Name, err)
				continue
			}
			return fmt.Errorf("handling file %d: %s: %w", i, f.Name, err)
		}
	}

	return nil
}

// decodeText decodes name and comment fields from hdr to UTF-8.
// Doesn't work if text is already encoded in UTF-8 or if z.TextEncoding is not specified.
func (z Zip) decodeText(hdr *zip.FileHeader) {
	if hdr.NonUTF8 && z.TextEncoding != "" {
		filename, err := decodeText(hdr.Name, z.TextEncoding)
		if err == nil {
			hdr.Name = filename
		}

		if hdr.Comment != "" {
			comment, err := decodeText(hdr.Comment, z.TextEncoding)
			if err == nil {
				hdr.Comment = comment
			}
		}
	}
}

func streamSizeBySeeking(s io.Seeker) (int64, error) {
	currentPosition, err := s.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("getting current offset: %w", err)
	}

	maxPosition, err := s.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("fast-forwarding to end: %w", err)
	}

	_, err = s.Seek(currentPosition, io.SeekStart)
	if err != nil {
		return 0, fmt.Errorf("returning to prior offset %d: %w", currentPosition, err)
	}

	return maxPosition, nil
}

// decodeText returns UTF-8 encoded text from the given charset.
// Thanks to @zxdvd for contributing non-UTF-8 encoding logic in
// #149, and to @pashifika for helping in #305.
func decodeText(input, charset string) (string, error) {
	if enc, ok := encodings[charset]; ok {
		return enc.NewDecoder().String(input)
	}

	return "", fmt.Errorf("unrecognized charset %s", charset)
}
