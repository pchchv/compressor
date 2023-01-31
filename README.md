# **compressor** [![Go Reference](https://pkg.go.dev/badge/github.com/pchchv/compressor.svg)](https://pkg.go.dev/github.com/pchchv/compressor)

 Easy creation and extraction of archives, as well as compression and decompression of files of different formats

# Supported archive formats

* **.zip**
* **.tar (including any compressed variants like .tar.gz)**
* **.rar (read-only)**
* **.7z (read-only)**

# Supported compression formats

* **brotli (.br)**
* **bzip2 (.bz2)**
* **flate (.zip)**
* **gzip (.gz)**
* **lz4 (.lz4)**
* **snappy (.sz)**
* **xz (.xz)**
* **zlib (.zz)**
* **zstandard (.zst)**

# Features

* Pure Go (no cgo)
* Multi-threaded Gzip
* Stream-oriented APIs
* Adjust compression levels
* Cross platform, static binary
* Create and extract archive files
* Compressing and decompressing files
* Inserting (adding) to .tar archives
* Open password protected RAR archives
* Extract only specific files from archives
* Read from password protected 7-Zip archives
* Supports numerous archive formats and compression.
* Automatically identify archive and compression formats:
	* By file name
	* By header
* Expandability (adding new formats simply by registering them)
* Automatically add compressed files to zip archives without recompressing
* Traverse directories, archive files, and any other files uniformly as [`io/fs`](https://pkg.go.dev/io/fs) file systems:
	* [`DirFS`](https://pkg.go.dev/github.com/pchchv/compressor/#DirFS)
	* [`FileFS`](https://pkg.go.dev/github.com/pchchv/compressor/#FileFS)
	* [`ArchiveFS`](https://pkg.go.dev/github.com/pchchv/compressor/#ArchiveFS)

# Using

```bash
$ go get github.com/pchchv/compressor
```

## *Create archive*

Creating archives can be done completely without using a real disk or storage device. All you need is a list of [`File`](https://pkg.go.dev/github.com/pchchv/compressor/#File) structures to transfer.

But creating archives from files on disk is very common, so you can use the function [`FilesFromDisk()`](https://pkg.go.dev/github.com/pchchv/compressor/#FilesFromDisk), which will help you map file names on disk to their paths in the archive. Then create and configure the format type.

In this example we add 4 files and a directory (which recursively includes its contents) to the .tar.gz archive:

```go
// map files on disk to their paths in the archive
files, err := compressor.FilesFromDisk(nil, map[string]string{
	"/path/on/disk/file1.txt": "file1.txt",
	"/path/on/disk/file2.txt": "subfolder/file2.txt",
	"/path/on/disk/file3.txt": "",
	"/path/on/disk/file4.txt": "subfolder/",
	"/path/on/disk/folder":    "Custom Folder",
})
if err != nil {
	return err
}

// create an output file in which to write
out, err := os.Create("example.tar.gz")
if err != nil {
	return err
}
defer out.Close()

// it is possible to use CompressedArchive type for gzip tarball
// (compression is not required, Tar can be used directly)
format := compressor.CompressedArchive{
	Compression: compressor.Gz{},
	Archival:    compressor.Tar{},
}

// create the archive
if err = format.Archive(context.Background(), out, files); err != nil {
	return err
}
```

The first parameter to `FilesFromDisk()` is an optional options structure that allows you to configure how to add files.

## *Extract archive*

Extract archive, extract **from** archive and traversing the archive are all the same function.

Just use the format type (e.g. `Zip`) to call `Extract()`. Pass the context, the input stream, the list of files you want to extract from the archive *(If you want all the files, pass nil the list of file paths.)*, and a callback function to handle each file. 


```go
// the type that will be used to read the input stream
format := compressor.Zip{}

// the list of files to be extracted from the archive
// any directories will include all of their contents unless returned by the fs.SkipDir handler
// (leave this parameter set to nil to output ALL files from the archive)
fileList := []string{"file1.txt", "subfolder"}

handler := func(ctx context.Context, f compressor.File) error {
	// file manipulation
	return nil
}

err := format.Extract(ctx, input, fileList, handler)
if err != nil {
	return err
}
```

## *Identifying formats*
Got an input stream with unknown content? No problem, the compressor can detect it. It will try to match based on the filename and/or header (which peeks at the stream):

```go
format, input, err := compressor.Identify("filename.tar.zst", input)
if err != nil {
	return err
}

//now you can make the type-assert format whatever you want
// don't forget to use the return stream to re-read the consumed bytes during Identify()

// extracting
if ex, ok := format.(compressor.Extractor); ok {
	// proceed to extract
}

// decompressing
if dc, ok := format.(compressor.Decompressor); ok {
	rc, err := dc.OpenReader(unknownFile)
	if err != nil {
		return err
	}
	defer rc.Close()

	// read from rc to get decompressed data
}
```

`Identify()` works by reading an arbitrary number of bytes from the beginning of the stream to check file headers. It buffers them and returns a new reader which lets you read them again.

## *Virtual file systems*

The use of any file (a real directory on disk, an archive, a compressed archive, or any other ordinary file) is uniform, no matter what it is.

Use compressor to easily create a file system:

```go
// the file name can be:
//  - folder ("~/Projects/go/src")
//  - archive ("example.zip")
//  - compressed archive ("example.tar.gz")
//  - plain file ("example.txt")
//  - compressed plain file ("example.txt.gz")
fsys, err := compressor.FileSystem(filename)
if err != nil {
	return err
}
```

This is a fully functional `fs.FS`, so you can open files and read directories no matter what file was entered.

For example, to open a specific file:

```go
f, err := fsys.Open("file")
if err != nil {
	return err
}

defer f.Close()
```

If you opened a regular file, you can read from it. If it's a compressed file, the reading are automatically decompressed.

If you opened a directory, you can list its contents:

```go
if dir, ok := f.(fs.ReadDirFile); ok {
	entries, err := dir.ReadDir(0) // 0 gets all entries, but you can pass > 0 to paginate
	if err != nil {
		return err
	}

	for _, e := range entries {
		fmt.Println(e.Name())
	}
}
```

Or get a directory listing this way:

```go
entries, err := fsys.ReadDir("Projescts")
if err != nil {
	return err
}

for _, e := range entries {
	fmt.Println(e.Name())
}
```

Or maybe you want to go through all or part of the filesystem, but skip the folder named `.git':

```go
err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if path == ".git" {
		return fs.SkipDir
	}

	fmt.Println("Go:", path, "Dir?", d.IsDir())
	return nil
})

if err != nil {
	return err
}
```

## *Use with `http.FileServer`*

It is possible to use with http.FileServer to browse archives and directories in the browser. However, because of the way http.FileServer works, do not use http.FileServer directly with compressed files. Instead, wrap it as follows:

```go
fileServer := http.FileServer(http.FS(archiveFS))
http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
	// disable range request
	writer.Header().Set("Accept-Ranges", "none")
	request.Header.Del("Range")
	
	// disable content-type sniffing
	ctype := mime.TypeByExtension(filepath.Ext(request.URL.Path))
	writer.Header()["Content-Type"] = nil
	if ctype != "" {
		writer.Header().Set("Content-Type", ctype)
	}
	fileServer.ServeHTTP(writer, request)
})
```

http.FileServer will, by default, attempt to define a Content-Type if it cannot be determined from the filename. To do this, the http package will attempt to read from the file and then Seek back to the beginning of the file, which the library cannot currently achieve. The same applies to Range queries. Seeking in archives is not currently supported by the compressor due to limitations in dependencies.

If content-type is desired, you can [register it yourself](https://pkg.go.dev/mime#AddExtensionType).

## *Compress data*

Compression formats allow recorders to open to compress data:

```go
// wrap underlying writer w
compr, err := compressor.Zstd{}.OpenWriter(w)
if err != nil {
	return err
}
defer compr.Close()

// writes to compr will be compressed
```

### Decompress data

Similarly, compression formats allow opening readers to decompress data:

```go
// wrap underlying reader r
decompressor, err := compressor.Brotli{}.OpenReader(r)
if err != nil {
	return err
}
defer decompressor.Close()

// reads from decompressor will be decompressed
```

## *Append to tarball*

Tar archives can be appended to without creating a whole new archive by calling `Insert()` on a tar stream. It is required that the tar-archive not be compressed (because of difficulties with changing compression dictionaries).

An example that adds a file to the tar archive on disk:

```go
tarball, err := os.OpenFile("example.tar", os.O_RDWR, 0644)
if err != nil {
	return err
}
defer tarball.Close()

// prepare a text file for the root of the archive
files, err := archiver.FilesFromDisk(nil, map[string]string{
	"~/lastminute.txt": "",
})

err := archiver.Tar{}.Insert(context.Background(), tarball, files)
if err != nil {
	return err
}
```