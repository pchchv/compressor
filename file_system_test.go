package compressor

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"testing"

	"github.com/pchchv/golog"
)

//go:embed test/test.zip
var testZIP []byte

func TestPathWithoutTopDir(t *testing.T) {
	for i, tc := range []struct {
		input, expect string
	}{
		{
			input:  "a/b/c",
			expect: "b/c",
		},
		{
			input:  "b/c",
			expect: "c",
		},
		{
			input:  "c",
			expect: "c",
		},
		{
			input:  "",
			expect: "",
		},
	} {
		if actual := pathWithoutTopDir(tc.input); actual != tc.expect {
			t.Errorf("Test %d (input=%s): Expected '%s' but got '%s'", i, tc.input, tc.expect, actual)
		}
	}
}

func ExampleArchiveFS_Stream() {
	fsys := ArchiveFS{
		Stream: io.NewSectionReader(bytes.NewReader(testZIP), 0, int64(len(testZIP))),
		Format: Zip{},
	}
	// You can serve the contents in a web server:
	http.Handle("/static", http.StripPrefix("/static",
		http.FileServer(http.FS(fsys))))

	// Or read the files using fs functions:
	dis, err := fsys.ReadDir(".")
	if err != nil {
		golog.FatalCheck(err)
	}

	for _, di := range dis {
		golog.Info(di.Name())
		b, err := fs.ReadFile(fsys, path.Join(".", di.Name()))
		if err != nil {
			golog.FatalCheck(err)
		}
		golog.Info(fmt.Sprint(bytes.Contains(b, []byte("require ("))))
	}
}
