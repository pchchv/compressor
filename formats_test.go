package compressor

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

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

func TestIdentifyCanAssessSmallOrNoContent(t *testing.T) {
	type args struct {
		stream io.ReadSeeker
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "should return nomatch for an empty stream",
			args: args{
				stream: bytes.NewReader([]byte{}),
			},
		},
		{
			name: "should return nomatch for a stream with content size less than known header",
			args: args{
				stream: bytes.NewReader([]byte{'a'}),
			},
		},
		{
			name: "should return nomatch for a stream with content size greater then known header size and not supported format",
			args: args{
				stream: bytes.NewReader([]byte(strings.Repeat("this is a txt content", 2))),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Identify("", tt.args.stream)
			if got != nil {
				t.Errorf("no Format expected for non archive and not compressed stream: found Format= %v", got.Name())
				return
			}
			if err.Error() != "no formats matched" {
				t.Fatalf("ErrNoMatch expected for non archive and not compressed stream: err :=%#v", err)
				return
			}
		})
	}
}

func TestIdentifyAndOpenZip(t *testing.T) {
	f, err := os.Open("test/test.zip")
	checkErr(t, err, "opening zip")
	defer f.Close()

	format, reader, err := Identify("test.zip", f)
	checkErr(t, err, "identifying zip")
	if format.Name() != ".zip" {
		t.Fatalf("unexpected format found: expected=.zip actual:%s", format.Name())
	}

	err = format.(Extractor).Extract(context.Background(), reader, nil, func(ctx context.Context, f File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		_, err = io.ReadAll(rc)
		return err
	})
	checkErr(t, err, "extracting zip")
}

func checkErr(t *testing.T, err error, msgFmt string, args ...interface{}) {
	t.Helper()
	if err == nil {
		return
	}
	args = append(args, err)
	t.Fatalf(msgFmt+": %s", args...)
}

func TestIdentifyDoesNotMatchContentFromTrimmedKnownHeaderHaving0Suffix(t *testing.T) {
	// Using the outcome of `n, err := io.ReadFull(stream, buf)` without minding n
	// may lead to a mis-characterization for cases with known header ending with 0x0
	// because the default byte value in a declared array is 0.
	// This test guards against those cases.
	tests := []struct {
		name   string
		header []byte
	}{
		{
			name:   "rar_v5.0",
			header: rarHeaderV5_0,
		},
		{
			name:   "rar_v1.5",
			header: rarHeaderV1_5,
		},
		{
			name:   "xz",
			header: xzHeader,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headerLen := len(tt.header)
			if headerLen == 0 || tt.header[headerLen-1] != 0 {
				t.Errorf("header expected to end with 0: header=%v", tt.header)
				return
			}
			headerTrimmed := tt.header[:headerLen-1]
			stream := bytes.NewReader(headerTrimmed)
			got, _, err := Identify("", stream)
			if got != nil {
				t.Errorf("no Format expected for trimmed know %s header: found Format= %v", tt.name, got.Name())
				return
			}
			if err.Error() != "no formats matched" {
				t.Fatalf("ErrNoMatch expected for for trimmed know %s header: err :=%#v", tt.name, err)
				return
			}
		})
	}
}
