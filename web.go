package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"html/template"
)

type pkgInfo struct {
	Package          string                                  `json:"package"`
	LatestVersion    string                                  `json:"latest_version"`
	ShortDescription string                                  `json:"short_description"`
	Description      string                                  `json:"description,omitempty"`
	License          string                                  `json:"license,omitempty"`
	Maintainer       string                                  `json:"maintainer,omitempty"`
	DownloadSize     int64                                   `json:"download_size,omitempty"`
	Homepage         string                                  `json:"homepage,omitempty"`
	Depends          []string                                `json:"depends,omitempty"`
	PreDepends       []string                                `json:"pre_depends,omitempty"`
	Recommends       []string                                `json:"recommends,omitempty"`
	Suggests         []string                                `json:"suggests,omitempty"`
	Enhances         []string                                `json:"enhances,omitempty"`
	Breaks           []string                                `json:"breaks,omitempty"`
	Conflicts        []string                                `json:"conflicts,omitempty"`
	Availability     map[string]map[string]map[string]string `json:"availability"` // version -> arch -> component -> download path
	Fields           map[string]string                       `json:"fields"`
	OtherDists       []string                                `json:"other_dists"`
}

// GenerateWeb generates the web interface. It must be called last.
func (r *Repo) GenerateWeb() error {
	webRoot := filepath.Join(r.OutRoot, "packages")
	err := os.Mkdir(webRoot, 0755)
	if err != nil {
		return fmt.Errorf("error making web dir: %v", webRoot)
	}

	packages := map[string]map[string]*pkgInfo{} // dist -> info from latest package
	archs, comps, dists := []string{}, []string{}, []string{}

	for distName, dist := range r.Dists {
		if !inSlice(dists, distName) {
			dists = append(dists, distName)
		}
		for compName, comp := range dist {
			if !inSlice(comps, compName) {
				comps = append(comps, compName)
			}
			for _, pkg := range comp {
				pkgName := pkg.Control.MustGet("Package")
				pkgVersion := pkg.Control.MustGet("Version")
				pkgArch := pkg.Control.MustGet("Architecture")
				if !inSlice(archs, pkgArch) {
					archs = append(archs, pkgArch)
				}

				if _, ok := packages[distName]; !ok {
					packages[distName] = map[string]*pkgInfo{}
				}
				if _, ok := packages[distName][pkgName]; !ok {
					packages[distName][pkgName] = &pkgInfo{
						Availability: map[string]map[string]map[string]string{},
						Fields:       map[string]string{},
						OtherDists:   []string{},
					}
				}

				if _, ok := (*packages[distName][pkgName]).Availability[pkgVersion]; !ok {
					(*packages[distName][pkgName]).Availability[pkgVersion] = map[string]map[string]string{}
				}
				if _, ok := (*packages[distName][pkgName]).Availability[pkgArch][pkgArch]; !ok {
					(*packages[distName][pkgName]).Availability[pkgVersion][pkgArch] = map[string]string{}
				}
				if _, ok := (*packages[distName][pkgName]).Availability[pkgVersion][pkgArch][compName]; !ok {
					(*packages[distName][pkgName]).Availability[pkgVersion][pkgArch][compName] = fmt.Sprintf("pool/%s/%s/%s/%s_%s_%s.deb", compName, getLetter(pkgName), pkgName, pkgName, pkgVersion, pkgArch)
				}

				if packages[distName][pkgName].Package == "" || anewer(pkgVersion, (*packages[distName][pkgName]).LatestVersion) {
					// fill in fields, as this is the newest version so far
					(*packages[distName][pkgName]).Package = pkgName
					(*packages[distName][pkgName]).LatestVersion = pkgVersion
					(*packages[distName][pkgName]).Description = pkg.Control.MightGet("Description")
					(*packages[distName][pkgName]).ShortDescription = strings.Split(pkg.Control.MightGet("Description"), "\n")[0]
					(*packages[distName][pkgName]).License = pkg.Control.MightGet("License")
					(*packages[distName][pkgName]).Maintainer = pkg.Control.MightGet("Maintainer")
					(*packages[distName][pkgName]).DownloadSize = pkg.Size
					(*packages[distName][pkgName]).Homepage = pkg.Control.MightGet("Homepage")
					(*packages[distName][pkgName]).Depends = splitList(pkg.Control.MightGet("Depends"))
					(*packages[distName][pkgName]).PreDepends = splitList(pkg.Control.MightGet("Pre-Depends"))
					(*packages[distName][pkgName]).Recommends = splitList(pkg.Control.MightGet("Recommends"))
					(*packages[distName][pkgName]).Suggests = splitList(pkg.Control.MightGet("Suggests"))
					(*packages[distName][pkgName]).Breaks = splitList(pkg.Control.MightGet("Breaks"))
					(*packages[distName][pkgName]).Enhances = splitList(pkg.Control.MightGet("Enhances"))
					(*packages[distName][pkgName]).Conflicts = splitList(pkg.Control.MightGet("Conflicts"))
					(*packages[distName][pkgName]).Fields = pkg.Control.Values
				}
			}
		}
	}

	sort.Strings(archs)
	sort.Strings(dists)
	sort.Strings(comps)

	for distName, tmp := range packages {
		for pkgName := range tmp {
			for _, checkDist := range dists {
				if _, ok := packages[checkDist][pkgName]; ok {
					(*packages[distName][pkgName]).OtherDists = append((*packages[distName][pkgName]).OtherDists, checkDist)
				}
			}
		}
	}

	repoData := map[string]interface{}{
		"packages": packages,
		"archs":    archs,
		"dists":    dists,
		"comps":    comps,
		"origin":   r.Origin,
	}

	buf, err := json.Marshal(repoData)
	if err != nil {
		return fmt.Errorf("error generating repo json: %v", err)
	}
	err = ioutil.WriteFile(filepath.Join(webRoot, "repo.json"), buf, 0644)
	if err != nil {
		return fmt.Errorf("error writing repo json: %v", err)
	}

	err = render(filepath.Join(webRoot, "index.html"), "Packages", "", distsTmpl, map[string]interface{}{
		"dists": dists,
	})
	if err != nil {
		return fmt.Errorf("error generating index.html: %v", err)
	}

	for distName, dist := range packages {
		webRootDist := filepath.Join(webRoot, distName)
		err := os.Mkdir(webRootDist, 0755)
		if err != nil {
			return fmt.Errorf("error generating dist/: %v", err)
		}

		err = render(filepath.Join(webRootDist, "index.html"), distName+" - Packages", "../", distTmpl, map[string]interface{}{
			"dist":     distName,
			"packages": dist,
		})
		if err != nil {
			return fmt.Errorf("error generating dist/index.html: %v", err)
		}

		for pkgName, pkg := range dist {
			webRootDistPkg := filepath.Join(webRootDist, pkgName)
			err := os.Mkdir(webRootDistPkg, 0755)
			if err != nil {
				return fmt.Errorf("error generating dist/pkg/: %v", err)
			}

			err = render(filepath.Join(webRootDistPkg, "index.html"), pkgName+" - Packages", "../../", pkgTmpl, map[string]interface{}{
				"dist":    distName,
				"pkgName": pkgName,
				"pkg":     pkg,
			})
			if err != nil {
				return fmt.Errorf("error generating dist/pkg/index.html: %v", err)
			}
		}
	}

	return nil
}

func anewer(a, b string) bool {
	va, err := NewVersion(a)
	if err != nil {
		return false
	}

	vb, err := NewVersion(b)
	if err != nil {
		return true
	}

	return va.GreaterThan(vb)
}

func splitList(l string) []string {
	ls := []string{}
	for _, i := range strings.Split(strings.Replace(l, ", ", ",", -1), ",") {
		if s := strings.TrimSpace(i); s != "" {
			ls = append(ls, s)
		}
	}
	return ls
}

func inSlice(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

func render(outfn string, title string, base string, t string, d interface{}) error {
	f, err := os.Create(outfn)
	if err != nil {
		return err
	}
	defer f.Close()
	return template.Must(template.Must(template.New("").Funcs(tmplFuncs).Parse(t)).Parse(baseTmpl)).Execute(f, map[string]interface{}{
		"title": title,
		"css":   baseCSS,
		"data":  d,
		"base":  base,
	})
}

var tmplFuncs = template.FuncMap{
	"br": func(s string) template.HTML {
		return template.HTML(strings.Replace(strings.Replace(template.HTMLEscapeString(s), "\r\n", "\n", -1), "\n", "<br />", -1))
	},
}

var baseTmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
	<meta http-equiv="X-UA-Compatible" content="ie=edge">
	<base href="{{.base}}" />
	<title>{{.title}}</title>
	<style>{{.css}}</style>
</head>
<body>
	<div class="nav">
		<div class="nav__section nav__section--left">
			<a href="index.html">Packages</a>
		</div>
		<div class="nav__section nav__section--left">
			<a href="../key.asc">GPG Key</a>
			<a href="search/">Search (coming soon)</a>
		</div>
	</div>

	<div class="content">
		{{template "content" .data}}
	</div>
</body>
</html>
`

var baseCSS = `
/* TODO: write css */
`

var distsTmpl = `
{{define "content"}}
	<div class="dist-cards">
		{{range $dist := .dists}}
			<a class="dist-card" href="{{$dist}}/">
				<div class="dist-card__name">{{$dist}}</div>
			</a>
		{{end}}
	</div>
{{end}}
`

var distTmpl = `
{{define "content"}}
	<div class="package-table">
		{{range $packageName, $package := .packages}}
			<a class="package-row" href="{{$.dist}}/{{$packageName}}/">
				<div class="package-row__col package-row__col--name">{{$packageName}}</div>
				<div class="package-row__col package-row__col--description">{{$package.ShortDescription}}</div>
			</a>
		{{end}}
	</div>
{{end}}
`

var pkgTmpl = `
{{define "content"}}
	<div class="package-info">
		{{.pkg.Description | br}}
		<!-- TODO: more stuff on page -->
	</div>
{{end}}
`
