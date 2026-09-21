package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
)

type declaredRoute struct {
	owner string
	route routeInfo
}

type declarationIndex struct {
	owners map[string]string
	types  map[string]*ast.TypeSpec
}

func collectDeclarations(files []*ast.File, controllers map[string]*controllerInfo) error {
	index := &declarationIndex{owners: make(map[string]string), types: make(map[string]*ast.TypeSpec)}
	for _, file := range files {
		if err := index.discover(file); err != nil {
			return err
		}
	}
	for _, file := range files {
		if err := index.collectRoutes(file, controllers); err != nil {
			return err
		}
	}
	for name, receiver := range index.owners {
		if receiver == "" {
			return fmt.Errorf("controller declaration %s has no routes; place var _ = %s.GET(\"/path\") before a method", name, name)
		}
	}
	return nil
}

func (index *declarationIndex) discover(file *ast.File) error {
	aliases := controllerImports(file)
	for _, declaration := range file.Decls {
		decl, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range decl.Specs {
			if err := index.discoverSpec(spec, aliases); err != nil {
				return err
			}
		}
	}
	return nil
}

func (index *declarationIndex) discoverSpec(spec ast.Spec, aliases map[string]bool) error {
	switch spec := spec.(type) {
	case *ast.TypeSpec:
		index.types[spec.Name.Name] = spec
	case *ast.ValueSpec:
		for _, value := range spec.Values {
			calls := declarationCalls(value)
			if !isControllerFactory(calls, aliases) {
				continue
			}
			if err := index.addOwner(spec, calls); err != nil {
				return err
			}
		}
	}
	return nil
}

func (index *declarationIndex) addOwner(spec *ast.ValueSpec, calls []*ast.CallExpr) error {
	if len(spec.Names) != 1 || len(spec.Values) != 1 || spec.Names[0].Name == "_" {
		return fmt.Errorf("Controllers must initialize one named package variable")
	}
	if err := validateModifiers(calls[1:]); err != nil {
		return err
	}
	index.owners[spec.Names[0].Name] = ""
	return nil
}

func controllerImports(file *ast.File) map[string]bool {
	aliases := make(map[string]bool)
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if path != "github.com/emmybxt/go-controller/v2/gocontroller" {
			continue
		}
		name := "gocontroller"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		aliases[name] = true
	}
	return aliases
}

func isControllerFactory(calls []*ast.CallExpr, aliases map[string]bool) bool {
	if len(calls) == 0 {
		return false
	}
	selector, ok := calls[0].Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Controllers" {
		return false
	}
	name, ok := selector.X.(*ast.Ident)
	return ok && aliases[name.Name]
}

// declarationCalls returns a factory/route call followed by its chained calls.
func declarationCalls(expr ast.Expr) []*ast.CallExpr {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	return append(declarationCalls(selector.X), call)
}

func validateModifiers(calls []*ast.CallExpr) error {
	for _, call := range calls {
		name := call.Fun.(*ast.SelectorExpr).Sel.Name
		if !isMiddlewareModifier(name) {
			return fmt.Errorf("unsupported declaration modifier %s; use Use, UseBefore, or UseAfter", name)
		}
	}
	return nil
}

func isMiddlewareModifier(name string) bool {
	return name == "Use" || name == "UseBefore" || name == "UseAfter"
}

func (index *declarationIndex) collectRoutes(file *ast.File, controllers map[string]*controllerInfo) error {
	for i, decl := range file.Decls {
		route, err := index.routeDeclaration(decl)
		if err != nil {
			return err
		}
		if route == nil {
			continue
		}
		method, err := followingMethod(file.Decls, i)
		if err != nil {
			return fmt.Errorf("%s.%s(%q): %w", route.owner, route.route.Method, route.route.Path, err)
		}
		if err := index.bindRoute(route, method, controllers); err != nil {
			return err
		}
	}
	return nil
}

func (index *declarationIndex) routeDeclaration(declaration ast.Decl) (*declaredRoute, error) {
	decl, ok := declaration.(*ast.GenDecl)
	if !ok || decl.Tok != token.VAR {
		return nil, nil
	}
	for _, raw := range decl.Specs {
		spec := raw.(*ast.ValueSpec)
		for _, expr := range spec.Values {
			route, err := index.parseRoute(expr)
			if err != nil {
				return nil, err
			}
			if route == nil {
				continue
			}
			if err := validateRouteVariable(decl, spec); err != nil {
				return nil, err
			}
			return route, nil
		}
	}
	return nil, nil
}

func validateRouteVariable(decl *ast.GenDecl, spec *ast.ValueSpec) error {
	if decl.Lparen.IsValid() || len(decl.Specs) != 1 {
		return fmt.Errorf("route declarations must use an ungrouped var _ = controller.METHOD(\"/path\")")
	}
	if len(spec.Names) != 1 || spec.Names[0].Name != "_" || len(spec.Values) != 1 {
		return fmt.Errorf("route declarations must use var _ = controller.METHOD(\"/path\")")
	}
	return nil
}

func (index *declarationIndex) parseRoute(expr ast.Expr) (*declaredRoute, error) {
	calls := declarationCalls(expr)
	if len(calls) == 0 {
		return nil, nil
	}
	selector := calls[0].Fun.(*ast.SelectorExpr)
	owner, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil, nil
	}
	if _, exists := index.owners[owner.Name]; !exists {
		return nil, nil
	}
	if isMiddlewareModifier(selector.Sel.Name) {
		return nil, nil
	}
	return parseDeclaredRoute(owner.Name, selector.Sel.Name, calls)
}

func parseDeclaredRoute(owner, method string, calls []*ast.CallExpr) (*declaredRoute, error) {
	if !isRouteVerb(method) {
		return nil, fmt.Errorf("%s.%s is not a supported route method", owner, method)
	}
	if err := validateModifiers(calls[1:]); err != nil {
		return nil, err
	}
	if len(calls[0].Args) == 0 {
		return nil, fmt.Errorf("%s.%s requires a quoted path", owner, method)
	}
	path, ok := calls[0].Args[0].(*ast.BasicLit)
	if !ok || path.Kind != token.STRING {
		return nil, fmt.Errorf("%s.%s path must be a string literal", owner, method)
	}
	value, err := strconv.Unquote(path.Value)
	if err != nil {
		return nil, err
	}
	return &declaredRoute{owner: owner, route: routeInfo{Method: method, Path: value}}, nil
}

func isRouteVerb(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE":
		return true
	default:
		return false
	}
}

func followingMethod(declarations []ast.Decl, index int) (*ast.FuncDecl, error) {
	if index+1 >= len(declarations) {
		return nil, fmt.Errorf("route declaration must immediately precede a controller method")
	}
	method, ok := declarations[index+1].(*ast.FuncDecl)
	if !ok || method.Recv == nil {
		return nil, fmt.Errorf("route declaration must immediately precede a controller method")
	}
	return method, nil
}

func (index *declarationIndex) bindRoute(route *declaredRoute, method *ast.FuncDecl, controllers map[string]*controllerInfo) error {
	receiver := receiverName(method.Recv.List[0].Type)
	spec := index.types[receiver]
	if spec == nil {
		return fmt.Errorf("%s: route requires a non-generic controller struct method", method.Name.Name)
	}
	if err := validateControllerType(spec); err != nil {
		return err
	}
	if previous := index.owners[route.owner]; previous != "" && previous != receiver {
		return fmt.Errorf("declaration %s cannot describe both %s and %s", route.owner, previous, receiver)
	}
	controller, err := declaredController(receiver, route.owner, controllers)
	if err != nil {
		return err
	}
	route.route.Handler = method.Name.Name
	if err := appendDeclaredRoute(controller, route.route); err != nil {
		return err
	}
	index.owners[route.owner] = receiver
	return nil
}

func declaredController(receiver, owner string, controllers map[string]*controllerInfo) (*controllerInfo, error) {
	controller := controllers[receiver]
	if controller == nil {
		controller = &controllerInfo{Name: receiver, Declaration: owner}
		controllers[receiver] = controller
	}
	if controller.Declaration != owner {
		return nil, fmt.Errorf("controller %s must use one declaration variable and cannot mix it with @Controller", receiver)
	}
	return controller, nil
}

func appendDeclaredRoute(controller *controllerInfo, route routeInfo) error {
	for _, existing := range controller.Routes {
		if existing.Method == route.Method && existing.Path == route.Path {
			return fmt.Errorf("controller %s declares duplicate route %s %s", controller.Name, route.Method, route.Path)
		}
	}
	controller.Routes = append(controller.Routes, route)
	return nil
}
