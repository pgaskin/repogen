package main

import (
	"fmt"
	"hash"
	"io"
	"os"
)

// MultiSum checksums r in multiple hash formats.
func MultiSum(r io.Reader, algs map[string]hash.Hash) (map[string]string, error) {
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

// MultiSumFile checksums fn in multipl hash formats.
func MultiSumFile(fn string, algs map[string]hash.Hash) (map[string]string, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s, err := MultiSum(f, algs)
	return s, err
}
