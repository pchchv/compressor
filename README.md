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

