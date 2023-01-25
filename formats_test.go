package compressor

import (
	"io"
	"strings"
	"testing"
)

// TestCompression
// checkErr
// TestIdentifyDoesNotMatchContentFromTrimmedKnownHeaderHaving0Suffix
// TestIdentifyCanAssessSmallOrNoContent
// compress
// archive
// newTmpTextFile
// TestIdentifyFindFormatByStreamContent
// TestIdentifyAndOpenZip

func TestRewindReader(t *testing.T) {
	data := "the header\nthe body\n"

	r := newRewindReader(strings.NewReader(data))

	buf := make([]byte, 10) // enough for 'the header'

	// test rewinding reads
	for i := 0; i < 10; i++ {
		r.rewind()
		n, err := r.Read(buf)
		if err != nil {
			t.Fatalf("Read failed: %s", err)
		}
		if string(buf[:n]) != "the header" {
			t.Fatalf("iteration %d: expected 'the header' but got '%s' (n=%d)", i, string(buf[:n]), n)
		}
	}

	// get the reader from header reader and make sure we can read all of the data out
	r.rewind()
	finalReader := r.reader()
	buf = make([]byte, len(data))
	n, err := io.ReadFull(finalReader, buf)
	if err != nil {
		t.Fatalf("ReadFull failed: %s (n=%d)", err, n)
	}

	if string(buf) != data {
		t.Fatalf("expected '%s' but got '%s'", string(data), string(buf))
	}
}
