package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

var routeVerbs = []string{"Get", "Post", "Put", "Patch", "Delete", "Head", "Options", "Connect", "Trace"}
var annotation = regexp.MustCompile(`^@([A-Za-z]+)\((.*)\)$`)

func parseControllers(dir string) (string, []controllerInfo, error) {
	pkg, files, err := parseFiles(dir)
	if err != nil {
		return "", nil, err
	}
	controllers := make(map[string]*controllerInfo)
	for _, file := range files {
		if err := collectControllers(file, controllers); err != nil {
			return "", nil, err
		}
	}
	for _, file := range files {
		if err := collectMethods(file, controllers); err != nil {
			return "", nil, err
		}
	}
	list := make([]controllerInfo, 0, len(controllers))
	for _, controller := range controllers {
		sort.SliceStable(controller.Routes, func(i, j int) bool {
			return routeLess(controller.Routes[i], controller.Routes[j])
		})
		list = append(list, *controller)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return pkg, list, nil
}

func parseFiles(dir string) (string, []*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, err
	}
	var files []*ast.File
	var pkg string
	for _, entry := range entries {
		include, err := includeFile(dir, entry.Name())
		if err != nil {
			return "", nil, err
		}
		if !include {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			return "", nil, err
		}
		if pkg != "" && pkg != file.Name.Name {
			return "", nil, fmt.Errorf("multiple packages in %s", dir)
		}
		pkg = file.Name.Name
		files = append(files, file)
	}
	if len(files) == 0 {
		return "", nil, fmt.Errorf("no Go package in %s", dir)
	}
	return pkg, files, nil
}

func includeFile(dir, name string) (bool, error) {
	if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, ".gen.go") {
		return false, nil
	}
	return build.Default.MatchFile(dir, name)
}

func collectControllers(file *ast.File, controllers map[string]*controllerInfo) error {
	for _, declaration := range file.Decls {
		decl, ok := declaration.(*ast.GenDecl)
		if !ok || decl.Tok != token.TYPE {
			continue
		}
		for _, spec := range decl.Specs {
			typeSpec := spec.(*ast.TypeSpec)
			tags := extractTags(decl.Doc, typeSpec.Doc)
			if _, ok := tags["Controller"]; !ok {
				continue
			}
			controller, err := parseController(typeSpec, tags)
			if err != nil {
				return err
			}
			controllers[controller.Name] = controller
		}
	}
	return nil
}

func parseController(spec *ast.TypeSpec, tags map[string]string) (*controllerInfo, error) {
	if _, ok := spec.Type.(*ast.StructType); !ok || spec.Assign.IsValid() {
		return nil, fmt.Errorf("controller %s must be a struct declaration", spec.Name.Name)
	}
	if spec.TypeParams != nil {
		return nil, fmt.Errorf("controller %s: generic controllers require explicit metadata", spec.Name.Name)
	}
	prefix, err := strconv.Unquote(tags["Controller"])
	if err != nil {
		return nil, fmt.Errorf("controller %s prefix: %w", spec.Name.Name, err)
	}
	middleware, err := parseMiddleware(tags["Use"])
	if err != nil {
		return nil, fmt.Errorf("controller %s middleware: %w", spec.Name.Name, err)
	}
	return &controllerInfo{Name: spec.Name.Name, Prefix: prefix, Middleware: middleware}, nil
}

func collectMethods(file *ast.File, controllers map[string]*controllerInfo) error {
	for _, decl := range file.Decls {
		method, ok := decl.(*ast.FuncDecl)
		if !ok || method.Recv == nil {
			continue
		}
		controller := controllers[receiverName(method.Recv.List[0].Type)]
		if controller == nil {
			continue
		}
		if method.Name.Name == "ControllerMetadata" {
			return fmt.Errorf("controller %s already implements ControllerMetadata; remove its @Controller annotation", controller.Name)
		}
		routes, err := parseMethod(method)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", controller.Name, method.Name.Name, err)
		}
		controller.Routes = append(controller.Routes, routes...)
	}
	return nil
}

func parseMethod(method *ast.FuncDecl) ([]routeInfo, error) {
	tags := extractTags(method.Doc)
	var routes []routeInfo
	for _, verb := range routeVerbs {
		raw, exists := tags[verb]
		if !exists {
			continue
		}
		path, err := strconv.Unquote(raw)
		if err != nil {
			return nil, fmt.Errorf("@%s path: %w", verb, err)
		}
		middleware, err := parseMiddleware(tags["Use"])
		if err != nil {
			return nil, err
		}
		routes = append(routes, routeInfo{strings.ToUpper(verb), path, method.Name.Name, middleware})
	}
	return routes, nil
}

func extractTags(groups ...*ast.CommentGroup) map[string]string {
	tags := make(map[string]string)
	for _, group := range groups {
		if group == nil {
			continue
		}
		for _, comment := range group.List {
			line := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			match := annotation.FindStringSubmatch(line)
			if len(match) == 3 {
				tags[match[1]] = strings.TrimSpace(match[2])
			}
		}
	}
	return tags
}

func parseMiddleware(raw string) ([]string, error) {
	expr, err := parser.ParseExpr("middleware(" + raw + ")")
	if err != nil {
		return nil, fmt.Errorf("@Use: %w", err)
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, fmt.Errorf("@Use requires a list of middleware expressions")
	}
	name, ok := call.Fun.(*ast.Ident)
	if !ok || name.Name != "middleware" {
		return nil, fmt.Errorf("@Use requires a list of middleware expressions")
	}
	if call.Ellipsis.IsValid() {
		return nil, fmt.Errorf("@Use requires individual middleware expressions, not a slice expansion")
	}
	var middleware []string
	for _, arg := range call.Args {
		var text bytes.Buffer
		if err := format.Node(&text, token.NewFileSet(), arg); err != nil {
			return nil, err
		}
		middleware = append(middleware, text.String())
	}
	return middleware, nil
}

func receiverName(expr ast.Expr) string {
	switch recv := expr.(type) {
	case *ast.Ident:
		return recv.Name
	case *ast.StarExpr:
		return receiverName(recv.X)
	default:
		return ""
	}
}

func routeLess(a, b routeInfo) bool {
	left, right := strings.Split(a.Path, "/"), strings.Split(b.Path, "/")
	for i := 0; i < min(len(left), len(right)); i++ {
		if segmentRank(left[i]) != segmentRank(right[i]) {
			return segmentRank(left[i]) < segmentRank(right[i])
		}
	}
	if len(left) != len(right) {
		return len(left) < len(right)
	}
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Method != b.Method {
		return a.Method < b.Method
	}
	return a.Handler < b.Handler
}

func segmentRank(segment string) int {
	if strings.ContainsAny(segment, "*+") {
		return 2
	}
	if strings.Contains(segment, ":") {
		return 1
	}
	return 0
}
