package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/css"
)

type pkgInfo struct {
	Package                 string                                  `json:"package"`
	LatestVersion           string                                  `json:"latest_version"`
	ShortDescription        string                                  `json:"short_description"`
	Description             string                                  `json:"description,omitempty"`
	License                 string                                  `json:"license,omitempty"`
	Maintainer              string                                  `json:"maintainer,omitempty"`
	MaintainerName          string                                  `json:"maintainer_name,omitempty"`
	MaintainerEmail         string                                  `json:"maintainer_email,omitempty"`
	Section                 string                                  `json:"section,omitempty"`
	DownloadSize            int64                                   `json:"download_size,omitempty"`
	Homepage                string                                  `json:"homepage,omitempty"`
	Depends                 []string                                `json:"depends,omitempty"`
	PreDepends              []string                                `json:"pre_depends,omitempty"`
	Recommends              []string                                `json:"recommends,omitempty"`
	Suggests                []string                                `json:"suggests,omitempty"`
	Enhances                []string                                `json:"enhances,omitempty"`
	Breaks                  []string                                `json:"breaks,omitempty"`
	Conflicts               []string                                `json:"conflicts,omitempty"`
	Availability            map[string]map[string]map[string]string `json:"availability"` // version -> arch -> component -> download path
	AvailabilityTableHeader []string                                `json:"availability_table_header"`
	AvailabilityTable       [][]map[string]string                   `json:"availability_table"` // [row][col][component] = link
	Fields                  map[string]string                       `json:"fields"`
	OtherDists              []string                                `json:"other_dists"`
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
					if r.MaintainerOverride != "" {
						(*packages[distName][pkgName]).Maintainer = r.MaintainerOverride
					}
					if m := regexp.MustCompile(`^(.+) <([^ ]+@[^ ]+)>$`).FindStringSubmatch(pkg.Control.MightGet("Maintainer")); len(m) == 3 {
						(*packages[distName][pkgName]).MaintainerName = m[1]
						(*packages[distName][pkgName]).MaintainerEmail = m[2]
					} else {
						(*packages[distName][pkgName]).MaintainerName = pkg.Control.MightGet("Maintainer")
					}
					(*packages[distName][pkgName]).DownloadSize = pkg.Size
					(*packages[distName][pkgName]).Homepage = pkg.Control.MightGet("Homepage")
					(*packages[distName][pkgName]).Depends = splitList(pkg.Control.MightGet("Depends"))
					(*packages[distName][pkgName]).PreDepends = splitList(pkg.Control.MightGet("Pre-Depends"))
					(*packages[distName][pkgName]).Recommends = splitList(pkg.Control.MightGet("Recommends"))
					(*packages[distName][pkgName]).Suggests = splitList(pkg.Control.MightGet("Suggests"))
					(*packages[distName][pkgName]).Breaks = splitList(pkg.Control.MightGet("Breaks"))
					(*packages[distName][pkgName]).Enhances = splitList(pkg.Control.MightGet("Enhances"))
					(*packages[distName][pkgName]).Conflicts = splitList(pkg.Control.MightGet("Conflicts"))
					(*packages[distName][pkgName]).Section = pkg.Control.MightGet("Section")
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

	for distName, tmp := range packages {
		for pkgName := range tmp {
			(*packages[distName][pkgName]).AvailabilityTableHeader = append([]string{""}, archs...)

			t := [][]map[string]string{}
			for pversion, parchs := range (*packages[distName][pkgName]).Availability {
				row := []map[string]string{{"version": pversion}}
				for _, arch := range archs {
					col := map[string]string{}
					for parch, pcomps := range parchs {
						if parch == arch {
							for pcomp, link := range pcomps {
								col[pcomp] = link
							}
						}
					}
					row = append(row, col)
				}
				t = append(t, row)
			}
			(*packages[distName][pkgName]).AvailabilityTable = t

			sort.Slice(t, func(i, j int) bool {
				return !anewer(t[i][0]["version"], t[j][0]["version"])
			})
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

	for _, dist := range dists {
		js, err := getSearchJS(dist, repoData)
		if err != nil {
			return fmt.Errorf("error generating search code: %v", err)
		}
		err = ioutil.WriteFile(filepath.Join(webRoot, "search."+dist+".js"), []byte(js), 0644)
		if err != nil {
			return fmt.Errorf("error writing repo search code: %v", err)
		}
	}

	err = render(filepath.Join(webRoot, "index.html"), "Packages", "", distsTmpl, map[string]interface{}{
		"dists":    dists,
		"packages": packages,
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
			"comps":    comps,
		})
		if err != nil {
			return fmt.Errorf("error generating dist/index.html: %v", err)
		}

		distPkgs := []string{}
		for pkgName := range dist {
			distPkgs = append(distPkgs, pkgName)
		}

		for pkgName, pkg := range dist {
			webRootDistPkg := filepath.Join(webRootDist, pkgName)
			err := os.Mkdir(webRootDistPkg, 0755)
			if err != nil {
				return fmt.Errorf("error generating dist/pkg/: %v", err)
			}

			err = render(filepath.Join(webRootDistPkg, "index.html"), pkgName+" - Packages", "../../", pkgTmpl, map[string]interface{}{
				"dist":         distName,
				"pkgName":      pkgName,
				"pkg":          pkg,
				"distPackages": distPkgs,
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
		"css":   template.CSS(baseCSS),
		"data":  d,
		"base":  base,
	})
}

var tmplFuncs = template.FuncMap{
	"br": func(s string) template.HTML {
		return template.HTML(strings.Replace(strings.Replace(template.HTMLEscapeString(s), "\r\n", "\n", -1), "\n", "<br />", -1))
	},
	"dependsToPkg": func(pkgSpec string) string {
		return strings.Split(pkgSpec, " ")[0]
	},
	"minifyCSS": func(in template.CSS) template.CSS {
		o, m := new(bytes.Buffer), minify.New()
		m.AddFunc("text/css", css.Minify)
		if m.Minify("text/css", o, strings.NewReader(string(in))) != nil {
			return in
		}
		return template.CSS(o.String())
	},
	"inSlice": inSlice,
}

var baseTmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
	<meta http-equiv="X-UA-Compatible" content="ie=edge" />
	<base href="{{.base}}" />
	<title>{{.title}}</title>
	<link href="https://cdnjs.cloudflare.com/ajax/libs/normalize/8.0.0/normalize.min.css" rel="stylesheet" />
	<style>{{.css | minifyCSS}}</style>
	<link href="https://fonts.googleapis.com/css?family=Bitter:400|Open+Sans:400,400i,700" rel="stylesheet" /> 
	<link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/4.7.0/css/font-awesome.min.css" rel="stylesheet" />
</head>
<body>
	<div class="nav">
		<div class="nav__section nav__section--left">
			<a class="nav__section__item" href="index.html">Home</a>
			{{if .data.dist}}
				<a class="nav__section__item" href="index.html">Dists</a>
				{{if .data.pkg}}
					<a class="nav__section__item" href="{{.data.dist}}/">{{.data.dist}}</a>
				{{end}}
			{{end}}
		</div>
		<div class="nav__section nav__section--right">
			{{if .data.dist}}
				<div class="search">
					<input type="text" class="search__query" autocomplete="off" placeholder="Search packages">
					<div class="search__results"></div>
				</div>
			{{end}}
			<a class="nav__section__item nav__section__item--gpg" href="../key.asc">GPG Key</a>
		</div>
	</div>

	<div class="content">
		{{template "content" .data}}
	</div>

	<div class="footer">
		Powered by <a href="https://github.com/geek1011/repogen">repogen</a>
	</div>

	{{if .data.dist}}
		<script src="search.{{.data.dist}}.js"></script>
	{{end}}
</body>
</html>
`

var baseCSS = `
html, body {
    padding: 0;
    margin: 0;
}

body {
    font-family: 'Open Sans', Helvetica, sans-serif;
    font-size: 14px;
    background: #fafafa;
    line-height: 1.42;
}

.nav {
    display: flex;
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
    background: #31447a;
    color: #fff;
}

.nav__section {
    flex: 0 0 auto;
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: flex-start;
    padding: 0 15px;
}

.nav__section__item {
    display: inline-block;
    vertical-align: middle;
    padding: 11px 15px;
}

.nav__section__item:link,
.nav__section__item:visited {
    color: inherit;
    text-decoration: none;
    cursor: pointer;
}

.nav__section__item:hover {
    background: #303b6b;
}

.nav__section__item.active {
    background: #526da4;
}

.package-info {
    display: block;
}

.package-info__header {
    display: block;
    background: #ebebeb;
    background: linear-gradient(to bottom, #fafafa 0%, #ebebeb 100%);
    padding: 15px 30px;
    border-bottom: 1px solid #e0e0e0;
}

.package-info__header__dist {
    display: block;
}

.package-info__header__name {
    display: block;
    font-family: Bitter, 'Open Sans', Helvetica, sans-serif;
    font-size: 28px;
    margin-bottom: 4px;
}

.package-info__header__shortdesc {
    display: block;
    font-size: 17px;
}

.package-info__body {
    display: block;
    margin: 0 30px;
}

.block {
    display: block;
    overflow: hidden;
    border-radius: 4px;
    border: 1px solid #ddd;
    margin: 20px 0;
    background: #fff;
    box-shadow: 0 1px 1px rgba(0,0,0,.05);
}

.block__title {
    display: block;
    font-family: Bitter, 'Open Sans', Helvetica, sans-serif;
    font-size: 16px;
    padding: 10px 15px;
    background-color: #f5f5f5;
    border-bottom: 1px solid #ddd;
}

.block__body {
    color: #333;
    padding: 10px 15px;
    overflow-x: auto;
    overflow-y: hidden;
}

.block__body.block__body--nopadding {
    padding: 0;
}

.block__body.block__body--monospace {
	font-family: monospace;
	white-space: nowrap;
}

.block__body__list {
    display: block;
    color: #555;
}

.block__body__list__item {
    display: block;
    border-bottom: 1px solid #ddd;
    padding: 10px 15px;
}

.block__body__list__item:last-child {
    border-bottom: none;
}

.block__body__list__item:link,
.block__body__list__item:visited {
    text-decoration: none;
    color: inherit;
    cursor: pointer;
    outline: none;
}

.block__body__list__item:link:hover {
    background: #f5f5f5;
}

.block__body__list__item--kv .block__body__list__item__key {
    display: block;
    color: #333;
    font-weight: bold;
}

.block__body__list__item--kv .block__body__list__item__value {
    display: block;
}

.block__body__list__item__icon {
    margin-right: 6px;
}

.depends-dot {
    display: inline-block;
    vertical-align: middle;
    border: 1px solid currentColor;
    border-radius: 8px;
    width: 8px;
    height: 8px;
    color: #777;
    margin-right: 6px;
}

.depends-dot.depends-dot--depends,
.depends-dot.depends-dot--pre-depends {
    color: #c70036;
    background: currentColor;
}

.depends-dot.depends-dot--recommends {
    border-radius: 0;
    color: #0040c7;
    background: currentColor;
}

.depends-dot.depends-dot--suggests {
    border-radius: 0;
    color: #1ca000;
    background: currentColor;
    transform: rotate(45deg);
}

.depends-dot.depends-dot--enhances {
    border-radius: 0;
    color: #ffa500;
}

.depends-dot.depends-dot--conflicts,
.depends-dot.depends-dot--breaks {
    border-radius: 0;
    color: #c70036;
}

.footer {
    background: #e1e1e1;
    box-shadow: inset 0 -1px 5px 0 rgba(0,0,0,0.1);
    margin-top: 20px;
    text-align: center;
    padding: 15px 30px;
    color: #444;
}

.version-table {
    display: table;
    border-collapse: collapse;
    width: 100%;
}

.version-table__row {
    display: table-row;
    border-bottom: 1px solid #ddd;
}

.version-table__row:last-child {
    border-bottom: none;
}

.version-table__row.version-table__row--header {
    font-weight: bold;
    color: #333;
}

.version-table__col {
    display: table-cell;
    padding: 8px 14px;
    border-right: 1px solid #ddd;
}

.version-table__col:last-child {
    border-right: none;
}

.version-table__col--version {
    width: 20%;
    min-width: 125px;
    max-width: 150px;
    border-right: 4px solid #ddd;
}

.version-table__col--arch {
    min-width: 100px;
    font-size: 12px;
}

.version-table__col a:link,
.version-table__col a:visited {
    display: inline-block;
    vertical-align: middle;
    white-space: no-wrap;
    text-decoration: none;
    outline: 0;
    color: #36b;
    background: #f0f0f0;
    border: 1px solid #eaeaea;
    padding: 2px 4px;
    margin: 2px 8px;
    margin-left: 0;
}

.version-table__col a:hover {
    text-decoration: underline;
}

::selection {
    background:#dae0ec;
}

::-moz-selection {
    background:#dae0ec;
}

.header {
    display: block;
    font-family: Bitter, 'Open Sans', Helvetica, sans-serif;
    margin: 15px 30px;
    font-size: 24px;
}

.header.header--center {
	text-align: center;
}

.dist-cards {
    display: block;
	margin: 15px 30px;
	text-align: center;
}

.dist-card,
.dist-card:link,
.dist-card:visited {
	text-align: left;
    display: block;
    margin-bottom: 15px;
    padding: 10px 15px;
    border: 1px solid #ddd;
    border-radius: 4px;
    color: inherit;
    text-decoration: none;
    background: #fff;
	box-shadow: 0 1px 1px rgba(0, 0, 0, .05);
	outline: 0;
	cursor: pointer;
}

.dist-card:hover {
    border: 1px solid #ccc;
    box-shadow: 0 1px 10px rgba(0, 0, 0, .10);
}

.dist-card__name {
    display: block;
    font-family: Bitter, 'Open Sans', Helvetica, sans-serif;
    font-size: 18px;
    color: #36b;
    font-weight: bold;
    margin-bottom: 4px;
}

.dist-card__name:hover {
    text-decoration: underline;
}

.dist-card__packages {
    display: block;
}

.nav__section__item.nav__section__item--gpg {
	display: none;
}

.search {
	margin-right: 18px;
	margin-left: 15px;
	width: 180px;
	transition: width .5s cubic-bezier(0.075, 0.82, 0.165, 1);
}

.search.search--focus {
	width: 250px;
}

.search__query {
	display: block;
	width: 100%;
	color: #fff;
	background: #223055;
	border: 1px solid #0e1322;
	background: rgba(0, 0, 0, 0.3);
	border: 1px solid rgba(0, 0, 0, 0.6);
	border-radius: 3px;
	outline: 0;
	padding: 4px 6px;
}

.search__query:focus {
	box-shadow: 0 0 4px #1c1c4780;
	box-shadow: 0 0 4px rgba(0, 0, 0, 0.5);
	border-color: #081a3e;
	border-color: rgba(0, 0, 0, 0.5);
}

.search .search__results {
	opacity: 0;
	margin-top: -10000px;
	transition: opacity .5s cubic-bezier(0.075, 0.82, 0.165, 1), margin 0s .5s;
}

.search.search--focus .search__results,
.search__results:hover {
	opacity: 1;
	margin-top: 4px;
	transition: opacity .5s cubic-bezier(0.075, 0.82, 0.165, 1);
}

.search__results {
	position: absolute;
	overflow: hidden;
	width: inherit;
	margin-top: 4px;
	background: #fff;
	border: 1px solid #CCD0DC;
	border-radius: 3px;
	color: #000;
}

.search__results:empty {
	display: none;
}

.search__results__none {
	padding: 4px 6px;
	font-size: 13px;
}

.search__results__result,
.search__results__result:link,
.search__results__result:visited {
	display: block;
	border-bottom: 1px solid #CCD0DC;
	padding: 4px 6px;
	font-size: 13px;
	color: inherit;
	outline: 0;
	text-decoration: none;
}

.search__results__result:last-child {
	border-bottom: none;
}

.search__results__result:hover,
.search__results__result.search__results__result--focus,
.search__results:hover .search__results__result.search__results__result--focus:hover {
	background: #f5f5f5;
}

.search__results:hover .search__results__result.search__results__result--focus {
	background: transparent;
}

.search__results__result:active {
	background: #ececec;
}

.search__results__result__package {
	font-weight: bold;
	text-overflow: ellipsis;
	overflow: hidden;
	white-space: nowrap;
}

.search__results__result__version {
	color: #555;
	float: right;
	text-overflow: ellipsis;
	overflow: hidden;
	white-space: nowrap;
}

.search__results__result__description {
	clear: both;
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}

@media only screen and (min-width: 768px) {
    .package-info__header__name {
        display: inline-block;
        vertical-align: middle;
        margin-bottom: 0;
    }
    
    .package-info__header__shortdesc:before {
        display: inline-block;
        content: '—';
        padding-right: .8em;
        padding-left: .8em;
    }
    
    .package-info__header__shortdesc {
        display: inline-block;
        vertical-align: middle;
        margin-bottom: 0;
    }
    
    .package-info__body {
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        justify-content: space-between;
    }
    
    .package-info__body__col {
        display: block;
        flex: 1;
    }
    
    .package-info__body__col.package-info__body__col--main {
        flex: 9;
    }
    
    .package-info__body__col.package-info__body__col--sidebar {
        flex: 3;
    }
    
    .package-info__body__col.package-info__body__col--sidebar {
        margin-left: 30px;
    }

    .block__body__list__item--kv .block__body__list__item__key {
        display: inline-block;
        vertical-align: top;
        width: 25%;
    }

    .block__body__list__item--kv .block__body__list__item__value {
        display: inline-block;
        vertical-align: top;
	}
	
	.dist-card,
	.dist-card:link,
	.dist-card:visited {
		display: inline-block;
		vertical-align: top;
		margin-right: 15px;
		margin-left: 15px;
		width: 150px;
	}

	.nav__section__item.nav__section__item--gpg {
		display: inline-block;
	}
	
	.search {
		width: 250px;
	}
	
	.search.search--focus {
		width: 300px;
	}
}

@media only screen and (max-width: 450px) {
	.nav {
		flex-direction: column;
		padding-bottom: 15px;
	}
}
`

var distsTmpl = `
{{define "content"}}
	<div class="header header--center">Distributions</div>
	<div class="dist-cards">
		{{range $dist := .dists}}
			<a class="dist-card" href="{{$dist}}/">
				<div class="dist-card__name">{{$dist}}</div>
				<div class="dist-card__packages">{{len (index $.packages $dist)}} packages</div>
			</a>
		{{end}}
	</div>
{{end}}
`

var distTmpl = `
{{define "content"}}
	<div class="header header--center">Packages in {{.dist}}</div>
	<div class="block" style="margin:15px 30px;">
		<div class="block__title">All Packages</div>
		<div class="block__body block__body--nopadding">
			<div class="block__body__list">
				{{range $packageName, $package := .packages}}
					<a class="block__body__list__item block__body__list__item--kv" href="{{$.dist}}/{{$packageName}}/">
						<div class="block__body__list__item__key">{{$packageName}}</div>
						<div class="block__body__list__item__value">{{$package.ShortDescription}}</div>
					</a>
				{{end}}
			</div>
		</div>
	</div>
	<div class="block" style="margin:15px 30px;">
		<div class="block__title">Installation</div>
		<div class="block__body block__body--monospace">
			# Add the repository key<br />
			<span style="color:#7a0874;font-weight:bold;">wget</span> <span style="color:#603">-O</span> - <span style="color:#f00;">'<span id="repo-key-url"><i>${REPO_URL}/key.asc</i></span>'</span> | <span style="color:#7a0874;font-weight:bold;">sudo apt-key add</span> - <br />
			<br />
			# Add the repository<br />
			<span style="color:#7a0874;font-weight:bold;">echo</span> <span style="color:#f00;">'deb <span id="repo-url"><i>${REPO_URL}</i></span> {{.dist}}{{range .comps}} {{.}}{{end}}'</span> | <span style="color:#7a0874;font-weight:bold;">sudo tee</span> <span style="color:#603">-a</span> /etc/apt/sources.list<br />
			<br />
			# Update package lists<br />
			<span style="color:#7a0874;font-weight:bold;">sudo apt update</span><br />
			
			<script>
				document.addEventListener("DOMContentLoaded", function () {
					document.getElementById("repo-url").innerHTML = "";
					document.getElementById("repo-url").innerText = window.location.toString().split("/packages")[0];
					document.getElementById("repo-key-url").innerHTML = "";
					document.getElementById("repo-key-url").innerText = window.location.toString().split("/packages")[0] + "/key.asc";
				});
			</script>
		</div> 
	</div>
{{end}}
`

var pkgTmpl = `
{{define "content"}}
	<div class="package-info">
		<div class="package-info__header">
			<div class="package-info__header__dist">dist: {{.dist}}</div>	
			<div class="package-info__header__name">{{.pkgName}}</div>	
			<div class="package-info__header__shortdesc">{{.pkg.ShortDescription}}</div>
		</div>
		<div class="package-info__body">
			<div class="package-info__body__col package-info__body__col--main">
				<div class="block">
					<div class="block__title">Available Versions</div>
					<div class="block__body block__body--nopadding">
						<div class="version-table">
							<div class="version-table__row version-table__row--header">
								{{range $i, $txt := .pkg.AvailabilityTableHeader}}
									{{if eq $i 0}}
										<div class="version-table__col version-table__col--version">Version</div>
									{{else}}
										<div class="version-table__col version-table__col--arch">{{$txt}}</div>
									{{end}}
								{{end}}
							</div>
							{{range $row := .pkg.AvailabilityTable}}
								<div class="version-table__row">
									{{range $i, $comps := $row}}
										{{if eq $i 0}}
											<div class="version-table__col version-table__col--version">{{index $comps "version"}}</div>
										{{else}}
											<div class="version-table__col version-table__col--arch">
												{{range $comp, $link := $comps}}
													<a href="../{{$link}}" title="Download">{{$comp}}</a>
												{{end}}
											</div>
										{{end}}
									{{end}}
								</div>
							{{end}}
						</div>
					</div>
				</div>
				<div class="block">
					<div class="block__title">Metadata</div>
					<div class="block__body block__body--nopadding">
						<div class="block__body__list">
							{{if .pkg.License}}
								<div class="block__body__list__item block__body__list__item--kv">
									<div class="block__body__list__item__key"><i class="fa fa-gavel block__body__list__item__icon"></i> License</div>
									<div class="block__body__list__item__value">{{.pkg.License}}</div>
								</div>
							{{end}}
							{{if .pkg.Maintainer}}
								<div class="block__body__list__item block__body__list__item--kv">
									<div class="block__body__list__item__key"><i class="fa fa-user block__body__list__item__icon"></i> Maintainer</div>
									<div class="block__body__list__item__value">
										<a href="mailto:{{.pkg.MaintainerEmail}}">{{.pkg.MaintainerName}}</a>
									</div>
								</div>
							{{end}}
							{{if .pkg.Section}}
								<div class="block__body__list__item block__body__list__item--kv">
									<div class="block__body__list__item__key"><i class="fa fa-sliders block__body__list__item__icon"></i> Section</div>
									<div class="block__body__list__item__value">{{.pkg.Section}}</div>
								</div>
							{{end}}
						</div>
					</div>
				</div>
				<div class="block">
					<div class="block__title">Description</div>
					<div class="block__body">
						{{.pkg.Description | br}}
					</div>
				</div>
				<div class="block">
					<div class="block__title">Dependencies</div>
					<div class="block__body block__body--nopadding">
						<div class="block__body__list">
							{{range $pkgspec := .pkg.Depends}}
								{{if (inSlice $.distPackages ($pkgspec | dependsToPkg))}}
									<a class="block__body__list__item" href="{{$.dist}}/{{$pkgspec | dependsToPkg}}"><span title="depends" class="depends-dot depends-dot--depends"></span> {{$pkgspec}}</a>
								{{else}}
									<div class="block__body__list__item"><span title="depends" class="depends-dot depends-dot--depends"></span> {{$pkgspec}}</div>
								{{end}}
							{{end}}
							{{range $pkgspec := .pkg.PreDepends}}
								{{if (inSlice $.distPackages ($pkgspec | dependsToPkg))}}
									<a class="block__body__list__item" href="{{$.dist}}/{{$pkgspec | dependsToPkg}}"><span title="pre-depends" class="depends-dot depends-dot--pre-depends"></span> {{$pkgspec}}</a>
								{{else}}
									<div class="block__body__list__item"><span title="pre-depends" class="depends-dot depends-dot--pre-depends"></span> {{$pkgspec}}</div>
								{{end}}
							{{end}}
							{{range $pkgspec := .pkg.Recommends}}
								{{if (inSlice $.distPackages ($pkgspec | dependsToPkg))}}
									<a class="block__body__list__item" href="{{$.dist}}/{{$pkgspec | dependsToPkg}}"><span title="recommends" class="depends-dot depends-dot--recommends"></span> {{$pkgspec}}</a>
								{{else}}
									<div class="block__body__list__item"><span title="recommends" class="depends-dot depends-dot--recommends"></span> {{$pkgspec}}</div>
								{{end}}
							{{end}}
							{{range $pkgspec := .pkg.Suggests}}
								{{if (inSlice $.distPackages ($pkgspec | dependsToPkg))}}
									<a class="block__body__list__item" href="{{$.dist}}/{{$pkgspec | dependsToPkg}}"><span title="suggests" class="depends-dot depends-dot--suggests"></span> {{$pkgspec}}</a>
								{{else}}
									<div class="block__body__list__item"><span title="suggests" class="depends-dot depends-dot--suggests"></span> {{$pkgspec}}</div>
								{{end}}
							{{end}}
							{{range $pkgspec := .pkg.Conflicts}}
								{{if (inSlice $.distPackages ($pkgspec | dependsToPkg))}}
									<a class="block__body__list__item" href="{{$.dist}}/{{$pkgspec | dependsToPkg}}"><span title="conflicts" class="depends-dot depends-dot--conflicts"></span> {{$pkgspec}}</a>
								{{else}}
									<div class="block__body__list__item"><span title="conflicts" class="depends-dot depends-dot--conflicts"></span> {{$pkgspec}}</div>
								{{end}}
							{{end}}
							{{range $pkgspec := .pkg.Breaks}}
								{{if (inSlice $.distPackages ($pkgspec | dependsToPkg))}}
									<a class="block__body__list__item" href="{{$.dist}}/{{$pkgspec | dependsToPkg}}"><span title="breaks" class="depends-dot depends-dot--breaks"></span> {{$pkgspec}}</a>
								{{else}}
									<div class="block__body__list__item"><span title="breaks" class="depends-dot depends-dot--breaks"></span> {{$pkgspec}}</div>
								{{end}}
							{{end}}
						</div>
					</div>
				</div>
			</div>
			<div class="package-info__body__col package-info__body__col--sidebar">
				<div class="block">
					<div class="block__title">Other Dists</div>
					<div class="block__body block__body--nopadding">
						<div class="block__body__list">
							{{range $otherDist := .pkg.OtherDists}}
								<a href="{{$otherDist}}/{{$.pkgName}}" class="block__body__list__item"><i class="fa fa-link block__body__list__item__icon"></i>
									{{if eq $.dist $otherDist}}
										<b>{{$otherDist}}</b>
									{{else}}
										{{$otherDist}}
									{{end}}
								</a>
							{{end}}
						</div>
					</div>
				</div>
				<div class="block">
					<div class="block__title">Links</div>
					<div class="block__body block__body--nopadding">
						<div class="block__body__list">
							{{if .pkg.Homepage}}
								<a href="{{.pkg.Homepage}}" class="block__body__list__item"><i class="fa fa-home block__body__list__item__icon"></i> Homepage</a>
							{{end}}
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
{{end}}
`
