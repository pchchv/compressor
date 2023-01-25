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
	f, err := os.Open("data/test.zip")
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
