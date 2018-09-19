package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"hash"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMultiSum(t *testing.T) {
	s, err := multiSum(strings.NewReader("test"), map[string]hash.Hash{
		"SHA256": sha256.New(),
		"SHA1":   sha1.New(),
		"MD5":    md5.New(),
	})
	assert.NoError(t, err, "should not error")
	assert.Equal(t, map[string]string{
		"SHA256": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		"SHA1":   "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3",
		"MD5":    "098f6bcd4621d373cade4e832627b4f6",
	}, s, "sums should be correct")
}
