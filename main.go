package main

import "io/ioutil"

func main() {
	maintainerOverride := "Patrick Gaskin <geek1011@outlook.com>" // optional
	origin := "Repository"                                        // optional
	description := "Test repository"                              // optional
	generateContents := true                                      //note: slower, but optional, as whole package needs to be read
	inRoot := "./in"
	outRoot := "./out"

	buf, _ := ioutil.ReadFile("/home/patrick/Downloads/patrick-g-gpg-key-backup.asc")

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

	// TODO: web interface, auto update, better error handling, pflag
}
