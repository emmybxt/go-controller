package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dir := flag.String("dir", ".", "package directory to scan")
	out := flag.String("out", "routes.gen.go", "output file name (must end in .gen.go)")
	check := flag.Bool("check", false, "check that generated output is current without writing")
	flag.Parse()
	if err := run(*dir, *out, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(dir, out string, check bool) error {
	if !strings.HasSuffix(out, ".gen.go") {
		return fmt.Errorf("output file must end in .gen.go")
	}
	pkg, controllers, err := parseControllers(dir)
	if err != nil {
		return err
	}
	if len(controllers) == 0 {
		return fmt.Errorf("no controller annotations or declarations found in %s", dir)
	}
	code, err := generateCode(pkg, controllers)
	if err != nil {
		return err
	}
	return writeOutput(filepath.Join(dir, out), code, check)
}

func writeOutput(path string, code []byte, check bool) error {
	if !check {
		return os.WriteFile(path, code, 0o644)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, code) {
		return fmt.Errorf("%s is stale; run go generate", path)
	}
	return nil
}
