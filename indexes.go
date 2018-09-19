package main

import (
	"fmt"
	"sort"
	"strings"
)

// GeneratePackageIndex generates a Packages file based on the provided debs.
func GeneratePackageIndex(debs []*Deb, maintainerOverride string) string {
	var b strings.Builder
	for _, d := range debs {
		c := d.Control.Clone()

		c.MoveToOrderStart("Package")

		if maintainerOverride != "" {
			c.Set("Maintainer", maintainerOverride)
		}

		c.Set("Size", fmt.Sprint(d.Size))
		for field, sum := range d.Sums {
			c.Set(field, sum)
		}

		//TODO: filename field

		b.WriteString(c.String() + "\n")
	}
	return b.String()
}

// GenerateContents generates a Contents file based on the provided debs.
func GenerateContents(debs []*Deb) string {
	var b strings.Builder
	contents := map[string][]string{}
	for _, d := range debs {
		for _, fn := range d.Contents {
			if _, ok := contents[fn]; !ok {
				contents[fn] = []string{}
			}

			qname := d.Control.MustGet("Package") // qname is the qualified package name [$SECTION/]$NAME
			if s, ok := d.Control.Get("Section"); ok {
				qname = s + "/" + qname
			}

			contents[fn] = append(contents[fn], qname)
		}
	}

	fns := []string{}
	for fn := range contents {
		fns = append(fns, fn)
	}
	sort.Strings(fns)

	for _, fn := range fns {
		b.WriteString(fmt.Sprintf("%-56s %s\n", fn, strings.Join(contents[fn], ",")))
	}
	return b.String()
}

// GenerateInRelease generates a Release file based on the provided info.
