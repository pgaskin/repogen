package main

func main() {
	maintainerOverride := "Patrick Gaskin <geek1011@outlook.com>"
	generateContents := true //note: slower, but optional, as whole package needs to be read
	inRoot := "./in"
	outRoot := "./out"

	r, err := NewRepo(inRoot, outRoot, generateContents, maintainerOverride)
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

	// TODO: web interface, auto update, better error handling, pflag
}
