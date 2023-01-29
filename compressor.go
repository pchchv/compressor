package compressor

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// File abstraction for interacting with archives.
type File struct {
	fs.FileInfo

	// The file header as used/provided by the archive format.
	Header interface{}

	// The path of the file as it appears in the archive.
	FileName string

	// For symbolic and hard links.
	// Not all archive formats are supported.
	LinkTarget string

	// A callback function that opens a file to read its contents.
	// The file must be closed when the reading is finished.
	// Not used for files that have no content (directories and links).
	Open func() (io.ReadCloser, error)
}

// FromDiskOptions specifies options for gathering files from the disk.
type FromDiskOptions struct {
	// If true, symbolic links will be dereferenced,
	// that is, the link will not be added as a link,
	// but what the link points to will be added as a file.
	FollowSymboliclinks bool

	// If true, some attributes of the file will not be saved.
	// The name, size, type and permissions will be saved.
	ClearAttributes bool
}

// noAttrFileInfo is used to zero some file attributes.
type noAttrFileInfo struct {
	fs.FileInfo
}

// skipList keeps a list of non-intersecting paths as long as its add method is used.
// Identical items are rejected, more specific paths are replaced with broader ones,
// and more specific paths won't be added when a broader one already exists in the list.
// Slashes are ignored.
type skipList []string

// FileHandler is a callback function that is used to handle files when reading them from an archive.
// It is similar to fs.WalkDirFunc. Handler functions that open files must not overlap or execute at the same time,
// since files can be read from the same sequential thread. Always close the file before returning it.
// If a special error value of fs.SkipDir is returned, the file directory (or the file itself,
// if it is a directory) will not be passed. Note that since the contents of an archive are not necessarily ordered,
// skipping directories requires memory, and skipping a large number of directories can lead to memory overruns.
// Any other error returned will abort the pass.
type FileHandler func(ctx context.Context, f File) error

func (f File) Stat() (fs.FileInfo, error) {
	return f.FileInfo, nil
}

// Mode preserves only the type and permission bits.
func (no noAttrFileInfo) Mode() fs.FileMode {
	return no.FileInfo.Mode() & (fs.ModeType | fs.ModePerm)
}

func (noAttrFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (noAttrFileInfo) Sys() interface{} {
	return nil
}

func (s *skipList) add(dir string) {
	var dontAdd bool
	trimmedDir := strings.TrimSuffix(dir, "/")

	for i := 0; i < len(*s); i++ {
		trimmedElem := strings.TrimSuffix((*s)[i], "/")

		if trimmedDir == trimmedElem {
			return
		}

		// don't add dir if a broader path already exists in the list
		if strings.HasPrefix(trimmedDir, trimmedElem+"/") {
			dontAdd = true
			continue
		}

		// if dir is broader than a path in the list, remove more specific path in list
		if strings.HasPrefix(trimmedElem, trimmedDir+"/") {
			*s = append((*s)[:i], (*s)[i+1:]...)
			i--
		}
	}
	if !dontAdd {
		*s = append(*s, dir)
	}
}

// FilesFromDisk returns a list of files by traversing the directories in a given filename map.
// The keys are the names on disk, and the values are the associated names in the archive.
// Map keys pointing to directories on disk will be looked up and added to the archive recursively,
// with the root in the named directory.  They must use a platform path separator
// (backslash in Windows; slash in all others). For convenience, map keys ending with a delimiter ('/', or '\' in Windows)
// will only list the contents of the folder without adding the folder itself to the archive.
// Map values should normally use a slash ('/') as a separator regardless of platform,
// since most archive formats standardize this rune as a directory separator for filenames in the archive.
// For convenience, map values that are an empty string are interpreted as the base filename (no path) in the root of the archive;
// and map values ending with a slash will use the base filename in the given archive folder.
// The files will be assembled according to the settings specified in the options.
// This function is mainly used when preparing a list of files to add to the archive.
func FilesFromDisk(options *FromDiskOptions, filenames map[string]string) (files []File, err error) {
	for rootOnDisk, rootInArchive := range filenames {
		walkErr := filepath.WalkDir(rootOnDisk, func(filename string, d fs.DirEntry, err error) error {
			var linkTarget string

			if err != nil {
				return err
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			nameInArchive := nameOnDiskToNameInArchive(filename, rootOnDisk, rootInArchive)
			// is the root folder, add its contents to the rootInArchive target folder
			if info.IsDir() && nameInArchive == "" {
				return nil
			}

			// handle symbolic links
			if isSymlink(info) {
				if options != nil && options.FollowSymboliclinks {
					// dereference symlinks
					filename, err = os.Readlink(filename)
					if err != nil {
						return fmt.Errorf("%s: readlink: %w", filename, err)
					}

					info, err = os.Stat(filename)
					if err != nil {
						return fmt.Errorf("%s: statting dereferenced symlink: %w", filename, err)
					}
				} else {
					// preserve symlinks
					linkTarget, err = os.Readlink(filename)
					if err != nil {
						return fmt.Errorf("%s: readlink: %w", filename, err)
					}
				}
			}

			// handle file attributes
			if options != nil && options.ClearAttributes {
				info = noAttrFileInfo{info}
			}

			file := File{
				FileInfo:   info,
				FileName:   nameInArchive,
				LinkTarget: linkTarget,
				Open: func() (io.ReadCloser, error) {
					return os.Open(filename)
				},
			}

			files = append(files, file)
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	return files, nil
}

// trimTopDir removes the top or first directory from the path.
// It expects a path with a forward slash.
// For example, "a/b/c" => "b/c".
func trimTopDir(dir string) string {
	if len(dir) > 0 && dir[0] == '/' {
		dir = dir[1:]
	}

	if pos := strings.Index(dir, "/"); pos >= 0 {
		return dir[pos+1:]
	}

	return dir
}

// topDir returns the top or first directory in the path.
// It expects a forward-slashed path.
// For example, "a/b/c" => "a".
func topDir(dir string) string {
	if len(dir) > 0 && dir[0] == '/' {
		dir = dir[1:]
	}

	if pos := strings.Index(dir, "/"); pos >= 0 {
		return dir[:pos]
	}

	return dir
}

// nameOnDiskToNameInArchive converts the file name from disk to the name in the archive,
// following the rules defined by FilesFromDisk. nameOnDisk is the full name of the file on disk,
// which is expected to be prefixed by rootOnDisk and will be placed in the rootInArchive folder in the archive.
func nameOnDiskToNameInArchive(nameOnDisk, rootOnDisk, rootInArchive string) string {
	if strings.HasSuffix(rootOnDisk, string(filepath.Separator)) {
		rootInArchive = trimTopDir(rootInArchive)
	} else if rootInArchive == "" {
		rootInArchive = filepath.Base(rootOnDisk)
	}

	if strings.HasSuffix(rootInArchive, "/") {
		rootInArchive += filepath.Base(rootOnDisk)
	}

	truncPath := strings.TrimPrefix(nameOnDisk, rootOnDisk)

	return path.Join(rootInArchive, filepath.ToSlash(truncPath))
}

// openAndCopyFile opens file for reading, copies its contents to w, then closes file.
func openAndCopyFile(file File, w io.Writer) error {
	fileReader, err := file.Open()
	if err != nil {
		return err
	}

	defer fileReader.Close()

	_, err = io.Copy(w, fileReader)
	return err
}

// fileIsIncluded returns true if the filename is included in the filenameList,
// i.e. it is in the list, its parent folder/path is in the list, or the list is nil.
func fileIsIncluded(filenameList []string, filename string) bool {
	// include all files if there is no specific list
	if filenameList == nil {
		return true
	}

	for _, fn := range filenameList {
		// exact matches are of course included
		if filename == fn {
			return true
		}

		// also consider the file included if its parent folder/path is in the list
		if strings.HasPrefix(filename, strings.TrimSuffix(fn, "/")+"/") {
			return true
		}
	}
	return false
}

func isSymlink(info fs.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}
