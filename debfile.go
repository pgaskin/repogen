package main

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kjk/lzma"
	"github.com/xi2/xz"
)

// Deb represents a deb archive.
type Deb struct {
	Control  *Control
	Contents []string
	Sums     map[string]string
	Size     int64
	Filename string
}

// NewDeb opens a deb archive.
func NewDeb(fn string, getContents bool) (*Deb, error) {
	d := Deb{}

	fi, err := os.Stat(fn)
	if err != nil {
		return nil, fmt.Errorf("error stat-ing deb file: %v", err)
	}
	d.Size = fi.Size()

	d.Filename, err = filepath.Abs(fn)
	if err != nil {
		return nil, fmt.Errorf("error resolving path to deb file %v", err)
	}

	f, err := os.Open(fn)
	if err != nil {
		return nil, fmt.Errorf("error opening deb file: %v", err)
	}
	defer f.Close()

	d.Sums, err = multiSum(f, map[string]hash.Hash{
		"SHA512": sha512.New(),
		"SHA256": sha256.New(),
		"SHA1":   sha1.New(),
		"MD5sum": md5.New(),
	})
	if err != nil {
		return nil, fmt.Errorf("error calculating checksums for deb file: %v", err)
	}
	f.Seek(0, 0)

	ar, err := NewAr(f)
	if err != nil {
		return nil, fmt.Errorf("error reading ar archive: %v", err)
	}

	for {
		h, err := ar.Next()
		if err == io.EOF {
			break
		}

		switch {
		case h.Name == "debian-binary":
			buf, err := ioutil.ReadAll(ar)
			if err != nil {
				return nil, fmt.Errorf("error reading debian-binary: %v", err)
			}
			if !bytes.HasPrefix(buf, []byte("2.0")) {
				return nil, fmt.Errorf("unknown debian-binary version: %s", string(buf))
			}
			continue
		case strings.HasPrefix(h.Name, "control.tar"):
			tr, err := openTar(strings.TrimRight(h.Name, "/"), ar)
			if err != nil {
				return nil, fmt.Errorf("error reading control archive: %v", err)
			}
			var foundControl bool
			for {
				th, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return nil, fmt.Errorf("error reading control archive: %v", err)
				}
				if path.Clean(th.Name) == "control" {
					buf, err := ioutil.ReadAll(tr)
					if err != nil {
						return nil, fmt.Errorf("error reading control archive: %v", err)
					}
					d.Control, err = NewControlFromString(string(buf))
					if err != nil {
						return nil, fmt.Errorf("error parsing control: %v", err)
					}
					foundControl = true
				}
			}
			if !foundControl {
				return nil, fmt.Errorf("no control file in control archive for deb")
			}
		case strings.HasPrefix(h.Name, "data.tar") && getContents:
			tr, err := openTar(strings.TrimRight(h.Name, "/"), ar)
			if err != nil {
				return nil, fmt.Errorf("error reading data archive: %v", err)
			}
			d.Contents = []string{}
			for {
				th, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return nil, fmt.Errorf("error reading data archive: %v", err)
				}
				if th.FileInfo().IsDir() {
					continue
				}
				d.Contents = append(d.Contents, filepath.Clean(th.Name))
			}
		}
	}

	if _, ok := d.Control.Get("Package"); !ok {
		panic("no Package field in control")
	}

	if _, ok := d.Control.Get("Architecture"); !ok {
		panic("no Architecture field in control")
	}

	if _, ok := d.Control.Get("Version"); !ok {
		panic("no Version field in control")
	}

	return &d, nil
}

var decompressors = map[string]func(io.Reader) (io.Reader, error){
	".tar": func(r io.Reader) (io.Reader, error) {
		return r, nil
	},
	".gz": func(r io.Reader) (io.Reader, error) {
		return gzip.NewReader(r)
	},
	".bz2": func(r io.Reader) (io.Reader, error) {
		return bzip2.NewReader(r), nil
	},
	".xz": func(r io.Reader) (io.Reader, error) {
		return xz.NewReader(r, 0)
	},
	".lzma": func(r io.Reader) (io.Reader, error) {
		return lzma.NewReader(r), nil
	},
}

func openTar(fn string, r io.Reader) (*tar.Reader, error) {
	d, ok := decompressors[filepath.Ext(fn)]
	if !ok {
		return nil, fmt.Errorf("unknown compression format %s", filepath.Ext(fn))
	}
	dr, err := d(r)
	if err != nil {
		return nil, fmt.Errorf("error decompressing data: %v", err)
	}
	return tar.NewReader(dr), nil
}

// multiSum checksums r in multiple hash formats.
func multiSum(r io.Reader, algs map[string]hash.Hash) (map[string]string, error) {
	var ws []io.Writer
	for _, alg := range algs {
		alg.Reset()
		ws = append(ws, alg)
	}
	hw := io.MultiWriter(ws...)

	_, err := io.Copy(hw, r)
	if err != nil {
		return nil, err
	}

	hm := map[string]string{}
	for n, alg := range algs {
		hm[n] = fmt.Sprintf("%x", alg.Sum(nil))
	}

	return hm, nil
}
