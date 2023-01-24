package compressor

// MatchResult returns true if the format was found either by name, by stream, or by both parameters.
// The name usually refers to searching by file extension,
// and the stream refers to reading the first few bytes of the stream (its header).
// Matching by stream is usually more reliable,
// because filenames do not always indicate the contents of files, if they exist at all.
type MatchResult struct {
	ByName,
	ByStream bool
}

// Matched returns true if a match was made by either name or stream.
func (mr MatchResult) Matched() bool {
	return mr.ByName || mr.ByStream
}
