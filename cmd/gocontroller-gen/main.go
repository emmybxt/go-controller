package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type controllerInfo struct {
	Name       string
	Prefix     string
	Middleware []string
	Routes     []routeInfo
}

type routeInfo struct {
	Method     string
	Path       string
	Handler    string
	Middleware []string
}

type routeAnnotation struct {
	Tag    string
	Method string
}

var supportedRouteAnnotations = []routeAnnotation{
	{Tag: "Get", Method: "GET"},
	{Tag: "Post", Method: "POST"},
	{Tag: "Put", Method: "PUT"},
	{Tag: "Patch", Method: "PATCH"},
	{Tag: "Delete", Method: "DELETE"},
	{Tag: "Options", Method: "OPTIONS"},
}

func main() {
	var dir string
	var out string
	flag.StringVar(&dir, "dir", ".", "package directory to scan")
	flag.StringVar(&out, "out", "zz_gocontroller_routes.gen.go", "output file name")
	flag.Parse()

	pkgName, controllers, err := parseControllers(dir)
	if err != nil {
		exitf("parse controllers: %v", err)
	}
	if len(controllers) == 0 {
		exitf("no annotated controllers found in %s", dir)
	}

	code := generateCode(pkgName, controllers)
	outPath := filepath.Join(dir, out)
	if err := os.WriteFile(outPath, []byte(code), 0o644); err != nil {
		exitf("write %s: %v", outPath, err)
	}
}

func parseControllers(dir string) (string, []controllerInfo, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		name := info.Name()
		if strings.HasSuffix(name, ".gen.go") || strings.HasSuffix(name, "_test.go") {
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

	packageNames := make([]string, 0, len(pkgs))
	for name := range pkgs {
		packageNames = append(packageNames, name)
	}
	sort.Strings(packageNames)
	pkg := pkgs[packageNames[0]]
	fileNames := make([]string, 0, len(pkg.Files))
	for name := range pkg.Files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	controllers := map[string]*controllerInfo{}
	for _, fileName := range fileNames {
		file := pkg.Files[fileName]
		for _, decl := range file.Decls {
			d, ok := decl.(*ast.GenDecl)
			if !ok || d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, isStruct := ts.Type.(*ast.StructType); !isStruct {
					continue
				}
				tags := extractTags(ts.Doc, d.Doc)
				controllerTag, ok := tags["Controller"]
				if !ok {
					continue
				}
				controller := &controllerInfo{Name: ts.Name.Name, Prefix: trimQuotes(controllerTag)}
				if use, ok := tags["Use"]; ok {
					controller.Middleware = parseUseList(use)
				}
				controllers[controller.Name] = controller
			}
		}
	}

	for _, fileName := range fileNames {
		file := pkg.Files[fileName]
		for _, decl := range file.Decls {
			method, ok := decl.(*ast.FuncDecl)
			if !ok || method.Recv == nil || method.Name == nil {
				continue
			}
			controller, ok := controllers[receiverName(method.Recv)]
			if !ok {
				continue
			}
			tags := extractTags(method.Doc)
			for _, annotation := range supportedRouteAnnotations {
				raw, ok := tags[annotation.Tag]
				if !ok {
					continue
				}
				route := routeInfo{Method: annotation.Method, Path: trimQuotes(raw), Handler: method.Name.Name}
				if use, ok := tags["Use"]; ok {
					route.Middleware = parseUseList(use)
				}
				controller.Routes = append(controller.Routes, route)
			}
		}
	}
	for _, controller := range controllers {
		sortRoutesBySpecificity(controller.Routes)
	}

	list := make([]controllerInfo, 0, len(controllers))
	for _, c := range controllers {
		if len(c.Routes) == 0 {
			continue
		}
		list = append(list, *c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return pkg.Name, list, nil
}

func sortRoutesBySpecificity(routes []routeInfo) {
	sort.SliceStable(routes, func(i, j int) bool {
		leftStatic, leftParams, leftWildcards, leftSegments := routeSpecificity(routes[i].Path)
		rightStatic, rightParams, rightWildcards, rightSegments := routeSpecificity(routes[j].Path)
		if leftStatic != rightStatic {
			return leftStatic > rightStatic
		}
		if leftParams != rightParams {
			return leftParams < rightParams
		}
		if leftWildcards != rightWildcards {
			return leftWildcards < rightWildcards
		}
		if leftSegments != rightSegments {
			return leftSegments > rightSegments
		}
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		if routes[i].Method != routes[j].Method {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Handler < routes[j].Handler
	})
}

func routeSpecificity(path string) (static, params, wildcards, segments int) {
	for _, segment := range strings.Split(strings.Trim(path, "/"), "/") {
		if segment == "" {
			continue
		}
		segments++
		switch {
		case segment == "*" || strings.HasPrefix(segment, "*"):
			wildcards++
		case strings.HasPrefix(segment, ":"):
			params++
		default:
			static++
		}
	}
	return static, params, wildcards, segments
}

func extractTags(groups ...*ast.CommentGroup) map[string]string {
	out := map[string]string{}
	re := regexp.MustCompile(`^@([A-Za-z]+)\((.*)\)$`)
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
			if len(m) != 3 {
				continue
			}
			out[m[1]] = strings.TrimSpace(m[2])
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

func parseUseList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func generateCode(pkg string, controllers []controllerInfo) string {
	var b strings.Builder
	b.WriteString("// Code generated by gocontroller-gen; DO NOT EDIT.\n")
	b.WriteString("\n")
	b.WriteString("package ")
	b.WriteString(pkg)
	b.WriteString("\n\n")
	b.WriteString("import \"github.com/emmybxt/go-controller/gocontroller\"\n\n")
	b.WriteString("func init() {\n")
	for _, c := range controllers {
		b.WriteString("\tgocontroller.RegisterGeneratedControllerMetadata((*")
		b.WriteString(c.Name)
		b.WriteString(")(nil), gocontroller.ControllerMetadata{\n")
		b.WriteString("\t\tPrefix: \"")
		b.WriteString(escape(c.Prefix))
		b.WriteString("\",\n")

		if len(c.Middleware) > 0 {
			b.WriteString("\t\tMiddleware: []gocontroller.Middleware{")
			for i, mw := range c.Middleware {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(mw)
			}
			b.WriteString("},\n")
		}

		b.WriteString("\t\tRoutes: []gocontroller.RouteMetadata{\n")
		for _, r := range c.Routes {
			b.WriteString("\t\t\tgocontroller.")
			b.WriteString(r.Method)
			b.WriteString("(\"")
			b.WriteString(escape(r.Path))
			b.WriteString("\", \"")
			b.WriteString(r.Handler)
			b.WriteString("\"")
			if len(r.Middleware) > 0 {
				b.WriteString(", ")
				for i, mw := range r.Middleware {
					if i > 0 {
						b.WriteString(", ")
					}
					b.WriteString(mw)
				}
			}
			b.WriteString("),\n")
		}
		b.WriteString("\t\t},\n")
		b.WriteString("\t})\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
