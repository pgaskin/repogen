package main

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ulikunitz/xz"
)

type Repo struct {
	InRoot             string
	OutRoot            string
	Dists              map[string]map[string][]*Deb // packages = Dists[dist][component]
	GenerateContents   bool
	MaintainerOverride string
}

func NewRepo(in, out string, generateContents bool, maintainerOverride string) (*Repo, error) {
	var err error

	if in, err = filepath.Abs(in); err != nil {
		return nil, fmt.Errorf("error resolving in path: %v", err)
	}

	if out, err = filepath.Abs(out); err != nil {
		return nil, fmt.Errorf("error resolving out path: %v", err)
	}

	if _, err := os.Stat(out); err == nil {
		return nil, errors.New("out must not exist")
	}

	return &Repo{
		InRoot:             in,
		OutRoot:            out,
		Dists:              map[string]map[string][]*Deb{},
		GenerateContents:   generateContents,
		MaintainerOverride: maintainerOverride,
	}, nil
}

// Clean removes the out dir.
func (r *Repo) Clean() {
	os.RemoveAll(r.OutRoot)
}

// Scan scans the in dir. Layout must be in/DIST/COMPONENT/*.deb.
func (r *Repo) Scan() error {
	dists := map[string]map[string][]*Deb{}

	dfs, err := ioutil.ReadDir(r.InRoot)
	if err != nil {
		return fmt.Errorf("could not list in dir: %v", err)
	}
	for _, dfi := range dfs {
		if !dfi.IsDir() {
			return fmt.Errorf("could not scan in dir: not a dir: %s", filepath.Join(r.InRoot, dfi.Name()))
		}
		distName, distRoot := dfi.Name(), filepath.Join(r.InRoot, dfi.Name())
		dists[distName] = map[string][]*Deb{}

		cfs, err := ioutil.ReadDir(distRoot)
		if err != nil {
			return fmt.Errorf("could not list in dir subdir: %v", err)
		}
		for _, cfi := range cfs {
			if !cfi.IsDir() {
				return fmt.Errorf("could not scan in dir: not a dir: %s", filepath.Join(r.InRoot, dfi.Name(), cfi.Name()))
			}
			compName, compRoot := cfi.Name(), filepath.Join(distRoot, cfi.Name())
			dists[distName][compName] = []*Deb{}

			pfs, err := ioutil.ReadDir(compRoot)
			if err != nil {
				return fmt.Errorf("could not list in dir subdir: %v", err)
			}
			for _, pfi := range pfs {
				if pfi.IsDir() || filepath.Ext(pfi.Name()) != ".deb" {
					return fmt.Errorf("could not scan in dir: not a deb file: %s", filepath.Join(r.InRoot, dfi.Name(), cfi.Name(), pfi.Name()))
				}
				pkgFname := filepath.Join(compRoot, pfi.Name())

				d, err := NewDeb(pkgFname, r.GenerateContents)
				if err != nil {
					return fmt.Errorf("could not read deb: %v", err)
				}
				dists[distName][compName] = append(dists[distName][compName], d)
			}
		}
	}

	r.Dists = dists
	return nil
}

// MakePool copies the deb files to the pool.
func (r *Repo) MakePool() error {
	poolRoot := filepath.Join(r.OutRoot, "pool")
	if err := os.MkdirAll(poolRoot, 0755); err != nil {
		return fmt.Errorf("error making pool dir: %v", err)
	}

	for _, dist := range r.Dists {
		for compName, comp := range dist {
			compRoot := filepath.Join(poolRoot, compName)
			if err := os.MkdirAll(compRoot, 0755); err != nil {
				return fmt.Errorf("error making component dir: %v", err)
			}
			for _, d := range comp {
				pkgName := d.Control.MustGet("Package")
				pkgArch := d.Control.MustGet("Architecture")
				pkgVer := d.Control.MustGet("Version")

				letterRoot := filepath.Join(compRoot, getLetter(pkgName))
				if err := os.MkdirAll(letterRoot, 0755); err != nil {
					return fmt.Errorf("error making letter dir: %v", err)
				}

				pkgRoot := filepath.Join(letterRoot, pkgName)
				if err := os.MkdirAll(pkgRoot, 0755); err != nil {
					return fmt.Errorf("error making pkg dir: %v", err)
				}

				pkgFName := filepath.Join(pkgRoot, fmt.Sprintf("%s_%s_%s.deb", pkgName, pkgVer, pkgArch))

				f, err := os.Open(d.Filename)
				if err != nil {
					return fmt.Errorf("error opening package file for copying: %v", err)
				}

				of, err := os.Create(pkgFName)
				if err != nil {
					f.Close()
					return fmt.Errorf("error opening output package file for copying: %v", err)
				}

				_, err = io.Copy(of, f)
				if err != nil {
					of.Close()
					f.Close()
					return fmt.Errorf("error writing package file: %v", err)
				}

				of.Close()
				f.Close()
			}
		}
	}
	return nil
}

// MakeDist generates the indexes.
func (r *Repo) MakeDist() error {
	distsRoot := filepath.Join(r.OutRoot, "dists")
	if err := os.MkdirAll(distsRoot, 0755); err != nil {
		return fmt.Errorf("error making dists dir: %v", err)
	}

	for distName, dist := range r.Dists {
		distRoot := filepath.Join(distsRoot, distName)
		if err := os.MkdirAll(distRoot, 0755); err != nil {
			return fmt.Errorf("error making dist dir: %v", err)
		}
		for compName, comp := range dist {
			compRoot := filepath.Join(distRoot, compName)
			if err := os.MkdirAll(compRoot, 0755); err != nil {
				return fmt.Errorf("error making component dir: %v", err)
			}
			archs := map[string][]*Deb{}
			for _, d := range comp {
				pkgArch := d.Control.MustGet("Architecture")
				if _, ok := archs[pkgArch]; !ok {
					archs[pkgArch] = []*Deb{}
				}
				archs[pkgArch] = append(archs[pkgArch], d)
			}
			for archName, arch := range archs {
				archRoot := filepath.Join(compRoot, "binary-"+archName)
				if err := os.MkdirAll(archRoot, 0755); err != nil {
					return fmt.Errorf("error making arch dir: %v", err)
				}
				var packages strings.Builder
				for _, d := range arch {
					c := d.Control.Clone()

					c.MoveToOrderStart("Package")

					if r.MaintainerOverride != "" {
						c.Set("Maintainer", r.MaintainerOverride)
					}

					c.Set("Size", fmt.Sprint(d.Size))
					for field, sum := range d.Sums {
						c.Set(field, sum)
					}

					//TODO: filename field

					packages.WriteString(c.String() + "\n")
				}
				packagesBytes := []byte(packages.String())

				gzf, err := os.Create(filepath.Join(archRoot, "Packages.gz"))
				if err != nil {
					return fmt.Errorf("error creating packages.gz file: %v", err)
				}
				gzw := gzip.NewWriter(gzf)
				if _, err := gzw.Write(packagesBytes); err != nil {
					gzw.Close()
					gzf.Close()
					return fmt.Errorf("error writing packages.gz file: %v", err)
				}
				gzw.Close()
				gzf.Close()

				xzf, err := os.Create(filepath.Join(archRoot, "Packages.xz"))
				if err != nil {
					return fmt.Errorf("error creating packages.xz file: %v", err)
				}
				xzw, err := xz.NewWriter(xzf)
				if err != nil {
					return fmt.Errorf("error creating xz writer for packages.xz file: %v", err)
				}
				if _, err := xzw.Write(packagesBytes); err != nil {
					xzw.Close()
					xzf.Close()
					return fmt.Errorf("error writing packages.xz file: %v", err)
				}
				xzw.Close()
				xzf.Close()
			}

			if r.GenerateContents {

				for archName, arch := range archs {
					var b strings.Builder
					contents := map[string][]string{}
					for _, d := range arch {
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

					contentsBytes := []byte(b.String())

					gzf, err := os.Create(filepath.Join(compRoot, "Contents-"+archName+".gz"))
					if err != nil {
						return fmt.Errorf("error creating contents.gz file: %v", err)
					}
					gzw := gzip.NewWriter(gzf)
					if _, err := gzw.Write(contentsBytes); err != nil {
						gzw.Close()
						gzf.Close()
						return fmt.Errorf("error writing contents.gz file: %v", err)
					}
					gzw.Close()
					gzf.Close()
				}
			}
		}
		//TODO: generate release, release.gpg, inrelease? files
	}
	return nil
}

func getLetter(pkg string) string {
	if strings.HasSuffix(pkg, "lib") {
		return pkg[:4]
	}
	return pkg[:1]
}
