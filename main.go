package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-zglob"
)

func main() {
	maintainerOverride := "Patrick Gaskin <geek1011@outlook.com>" // optional
	origin := "Repository"                                        // optional
	description := "Test repository"                              // optional
	generateContents := true                                      //note: slower, but optional, as whole package needs to be read
	inRoot := "./in"
	outRoot := "./out"
	watch := true
	watchInterval := time.Second

	buf, _ := ioutil.ReadFile("/home/patrick/Downloads/patrick-g-gpg-key-backup.asc")

	if _, err := os.Stat(outRoot); err == nil {
		panic("out must not exist")
	}

	var ls string
	for {
		for {
			if !watch {
				break
			}

			fs, err := zglob.Glob(filepath.Join(inRoot, "**", "*.deb"))
			if err != nil {
				panic(err)
			}

			sort.Strings(fs)

			if s := fmt.Sprintf("%x", sha256sum([]byte(strings.Join(fs, ";")))); ls != s {
				ls = s
				break
			}

			time.Sleep(watchInterval)
		}

		fmt.Println("updating repo")

		os.RemoveAll(outRoot)
		r, err := NewRepo(inRoot, outRoot, generateContents, maintainerOverride, origin, description, string(buf))
		if err != nil {
			panic(err)
		}

		err = r.Scan()
		if err != nil {
			panic(err)
		}

		err = r.MakePool()
		if err != nil {
			panic(err)
		}

		err = r.MakeDist()
		if err != nil {
			panic(err)
		}

		err = r.MakeRoot()
		if err != nil {
			panic(err)
		}

		if !watch {
			break
		}

		fmt.Println("waiting for changes")
	}

	// TODO: web interface, auto update, better error handling, pflag
}
