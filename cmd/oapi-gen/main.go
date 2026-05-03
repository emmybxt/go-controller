package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strings"
)

type oapiRoute struct {
	Method     string
	Path       string
	Handler    string
	Summary    string
	Deprecated bool
}

type oapiController struct {
	Name   string
	Prefix string
	Routes []oapiRoute
}

func main() {
	var dir string
	var out string
	var title string
	var version string
	var host string
	var scheme string

	flag.StringVar(&dir, "dir", ".", "package directory to scan")
	flag.StringVar(&out, "out", "openapi.json", "output file name")
	flag.StringVar(&title, "title", "API", "API title")
	flag.StringVar(&version, "version", "1.0.0", "API version")
	flag.StringVar(&host, "host", "localhost:8080", "API host")
	flag.StringVar(&scheme, "scheme", "http", "API scheme (http or https)")
	flag.Parse()

	pkgName, controllers, err := parseControllers(dir)
	if err != nil {
		exitf("parse controllers: %v", err)
	}
	if len(controllers) == 0 {
		exitf("no annotated controllers found in %s", dir)
	}

	spec := generateOpenAPI(title, version, host, scheme, pkgName, controllers)

	outPath := out
	if !strings.HasPrefix(outPath, "/") {
		outPath = dir + "/" + out
	}
	if err := os.WriteFile(outPath, []byte(spec), 0o644); err != nil {
		exitf("write %s: %v", outPath, err)
	}

	fmt.Fprintf(os.Stderr, "generated OpenAPI spec at %s\n", outPath)
}

func parseControllers(dir string) (string, []oapiController, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		name := info.Name()
		if strings.HasSuffix(name, ".gen.go") {
			return false
		}
		return strings.HasSuffix(name, ".go")
	}, parser.ParseComments)
	if err != nil {
		return "", nil, err
	}

	if len(pkgs) == 0 {
		return "", nil, fmt.Errorf("no package found")
	}

	var pkg *ast.Package
	for _, p := range pkgs {
		pkg = p
		break
	}

	controllers := map[string]*oapiController{}
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					_, isStruct := ts.Type.(*ast.StructType)
					if !isStruct {
						continue
					}
					tags := extractTags(ts.Doc, d.Doc)
					controllerTag, ok := tags["Controller"]
					if !ok {
						continue
					}
					c := &oapiController{
						Name:   ts.Name.Name,
						Prefix: trimQuotes(controllerTag),
					}
					if summary, ok := tags["Summary"]; ok {
						c.Name = trimQuotes(summary)
					}
					controllers[ts.Name.Name] = c
				}
			case *ast.FuncDecl:
				if d.Recv == nil || d.Name == nil {
					continue
				}
				recvType := receiverName(d.Recv)
				if recvType == "" {
					continue
				}
				ctrl, ok := controllers[recvType]
				if !ok {
					continue
				}
				tags := extractTags(d.Doc)
				for _, verb := range []string{"Get", "Post", "Put", "Delete", "Patch", "Options"} {
					raw, ok := tags[verb]
					if !ok {
						continue
					}
					route := oapiRoute{
						Method:  strings.ToUpper(verb),
						Path:    trimQuotes(raw),
						Handler: d.Name.Name,
					}
					if summary, ok := tags["Summary"]; ok {
						route.Summary = trimQuotes(summary)
					}
					if _, ok := tags["Deprecated"]; ok {
						route.Deprecated = true
					}
					ctrl.Routes = append(ctrl.Routes, route)
				}
			}
		}
	}

	list := make([]oapiController, 0, len(controllers))
	for _, c := range controllers {
		if len(c.Routes) == 0 {
			continue
		}
		sort.Slice(c.Routes, func(i, j int) bool { return c.Routes[i].Path < c.Routes[j].Path })
		list = append(list, *c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return pkg.Name, list, nil
}

func generateOpenAPI(title, version, host, scheme, pkgName string, controllers []oapiController) string {
	spec := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":   title,
			"version": version,
		},
		"servers": []map[string]any{
			{
				"url": scheme + "://" + host,
			},
		},
		"paths": map[string]any{},
	}

	paths := spec["paths"].(map[string]any)

	for _, ctrl := range controllers {
		for _, route := range ctrl.Routes {
			fullPath := ctrl.Prefix + route.Path
			if fullPath == "" || fullPath == "/" {
				fullPath = "/"
			}
			fullPath = normalizePath(fullPath)

			operation := map[string]any{
				"operationId": route.Handler,
				"summary":     route.Summary,
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Successful response",
					},
				},
			}

			if route.Deprecated {
				operation["deprecated"] = true
			}

			method := strings.ToLower(route.Method)
			if _, ok := paths[fullPath]; !ok {
				paths[fullPath] = map[string]any{}
			}
			paths[fullPath].(map[string]any)[method] = operation
		}
	}

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		exitf("marshal spec: %v", err)
	}

	return string(data) + "\n"
}

func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimRight(p, "/")
	if p == "" {
		return "/"
	}
	return p
}

func extractTags(groups ...*ast.CommentGroup) map[string]string {
	out := map[string]string{}
	re := regexp.MustCompile(`^@([A-Za-z]+)\((.*)\)$`)
	boolRe := regexp.MustCompile(`^@([A-Za-z]+)$`)
	for _, group := range groups {
		if group == nil {
			continue
		}
		for _, c := range group.List {
			line := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			if !strings.HasPrefix(line, "@") {
				continue
			}
			m := re.FindStringSubmatch(line)
			if len(m) == 3 {
				out[m[1]] = strings.TrimSpace(m[2])
				continue
			}
			m2 := boolRe.FindStringSubmatch(line)
			if len(m2) == 2 {
				out[m2[1]] = "true"
			}
		}
	}
	return out
}

func receiverName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	t := recv.List[0].Type
	switch v := t.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		if id, ok := v.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\"")
	s = strings.TrimSuffix(s, "\"")
	s = strings.TrimPrefix(s, "`")
	s = strings.TrimSuffix(s, "`")
	return s
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
