package compressor

import (
	"context"
	"io"
)

// The format is either an archive or a compression format.
type Format interface {
	// Name returns the name of the format.
	Name() string

	// Match returns true if the given name/stream is recognized. One of the arguments is optional:
	// the filename can be empty if you are working with an unnamed stream,
	// or the stream can be empty if you are working with just the filename.
	// The filename should consist only of the filename, not the path component,
	// and is usually used to search by file extension.
	// However, it is preferable to perform a read stream search.
	// Match reads only as many bytes as necessary to determine the match.
	// To save the stream when matching,
	// you must either buffer what Match reads or search for the last position before calling Match.
	Match(filename string, stream io.Reader) (MatchResult, error)
}

// Compressor can compress data by wrapping a writer.
type Compressor interface {
	// OpenWriter wraps w with a new writer that compresses what is written.
	// The writer must be closed when writing is finished.
	OpenWriter(w io.Writer) (io.WriteCloser, error)
}

// Decompressor can decompress data by wrapping a reader.
type Decompressor interface {
	// OpenReader wraps r with a new reader that decompresses what is read.
	// The reader must be closed when reading is finished.
	OpenReader(r io.Reader) (io.ReadCloser, error)
}

// Compression is a compression format with both compress and decompress methods.
type Compression interface {
	Format
	Compressor
	Decompressor
}

// Archiver can create a new archive.
type Archiver interface {
	// Archive writes an archive file to output with the given files.
	// Context cancellation must be honored.
	Archive(ctx context.Context, output io.Writer, files []File) error
}

// ArchiverAsync is an Archiver that can also create archives asynchronously,
// pumping files into the channel as they are discovered.
type ArchiverAsync interface {
	Archiver

	// Use ArchiveAsync if you cannot pre-assemble a list of all files for the archive.
	// Close the file channel after all files have been sent.
	ArchiveAsync(ctx context.Context, output io.Writer, files <-chan File) error
}

// Extractor can extract files from an archive.
type Extractor interface {
	// Extract reads the files at pathsInArchive from sourceArchive.
	// If pathsInArchive is nil, all files are extracted without restriction.
	// If pathsInArchive is empty, the files are not extracted.
	// If paths refer to a directory, all files in it are extracted.
	// Extracted files are passed to the handleFile callback for processing.
	// The context cancellation must be honored.
	Extract(ctx context.Context, sourceArchive io.Reader, pathsInArchive []string, handleFile FileHandler) error
}

// Inserter can insert files into an existing archive.
type Inserter interface {
	// Context cancellation must be honored.
	Insert(ctx context.Context, archive io.ReadWriteSeeker, files []File) error
}
