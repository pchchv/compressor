package compressor

import (
	"reflect"
	"testing"
)

func TestSkipList(t *testing.T) {
	for i, tc := range []struct {
		start  skipList
		add    string
		expect skipList
	}{
		{
			start:  skipList{"a", "b", "c"},
			add:    "d",
			expect: skipList{"a", "b", "c", "d"},
		},
		{
			start:  skipList{"a", "b", "c"},
			add:    "b",
			expect: skipList{"a", "b", "c"},
		},
		{
			start:  skipList{"a", "b", "c"},
			add:    "b/c", // don't add because b implies b/c
			expect: skipList{"a", "b", "c"},
		},
		{
			start:  skipList{"a", "b", "c"},
			add:    "b/c/", // effectively same as above
			expect: skipList{"a", "b", "c"},
		},
		{
			start:  skipList{"a", "b/", "c"},
			add:    "b", // effectively same as b/
			expect: skipList{"a", "b/", "c"},
		},
		{
			start:  skipList{"a", "b/c", "c"},
			add:    "b", // replace b/c because b is broader
			expect: skipList{"a", "c", "b"},
		},
	} {
		start := make(skipList, len(tc.start))
		copy(start, tc.start)

		tc.start.add(tc.add)

		if !reflect.DeepEqual(tc.start, tc.expect) {
			t.Errorf("Test %d (start=%v add=%v): expected %v but got %v",
				i, start, tc.add, tc.expect, tc.start)
		}
	}
}

func TestTrimTopDir(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "a/b/c", want: "b/c"},
		{input: "a", want: "a"},
		{input: "abc/def", want: "def"},
		{input: "/abc/def", want: "def"},
	} {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got := trimTopDir(tc.input)
			if got != tc.want {
				t.Errorf("want: '%s', got: '%s')", tc.want, got)
			}
		})
	}
}

func TestTopDir(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "a/b/c", want: "a"},
		{input: "a", want: "a"},
		{input: "abc/def", want: "abc"},
		{input: "/abc/def", want: "abc"},
	} {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got := topDir(tc.input)
			if got != tc.want {
				t.Errorf("want: '%s', got: '%s')", tc.want, got)
			}
		})
	}
}
