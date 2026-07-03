// Package structural holds architecture-enforcement tests. They parse the repository's Go source and
// assert the dependency-direction and purity rules described in ARCHITECTURE.md. These are executable
// invariants: a violation fails the build, not a review.
package structural

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	modulePrefix        = "github.com/oernster/pigeonpost/"
	domainPkg           = modulePrefix + "internal/domain"
	applicationPkg      = modulePrefix + "internal/application"
	infrastructurePkg   = modulePrefix + "internal/infrastructure"
	maxLinesPerFile     = 400
	wailsImportFragment = "wailsapp/wails"
)

type goFile struct {
	relDir  string
	pkg     string
	imports []string
	lines   int
	fset    *token.FileSet
	ast     *ast.File
}

var excludedDirs = map[string]bool{
	"frontend":     true,
	"build":        true,
	"node_modules": true,
	".git":         true,
}

func scanRepo(t *testing.T) []goFile {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var files []goFile
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if excludedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		parsed, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return perr
		}
		content, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, path)

		imports := make([]string, 0, len(parsed.Imports))
		for _, imp := range parsed.Imports {
			imports = append(imports, strings.Trim(imp.Path.Value, `"`))
		}
		files = append(files, goFile{
			relDir:  filepath.ToSlash(filepath.Dir(rel)),
			pkg:     parsed.Name.Name,
			imports: imports,
			lines:   strings.Count(string(content), "\n") + 1,
			fset:    fset,
			ast:     parsed,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan repo: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no Go files scanned; check repo root resolution")
	}
	return files
}

func inPackage(f goFile, dir string) bool {
	return f.relDir == dir || strings.HasPrefix(f.relDir, dir+"/")
}

func TestDomainHasNoOutwardImports(t *testing.T) {
	for _, f := range scanRepo(t) {
		if !inPackage(f, "internal/domain") {
			continue
		}
		for _, imp := range f.imports {
			if strings.HasPrefix(imp, modulePrefix) {
				t.Errorf("%s imports %q: the domain must not depend on any other project package", f.relDir, imp)
			}
			if strings.Contains(imp, wailsImportFragment) {
				t.Errorf("%s imports Wails %q: the domain must not depend on the UI framework", f.relDir, imp)
			}
		}
	}
}

func TestDomainIsPure(t *testing.T) {
	forbidden := map[string]bool{
		"os": true, "log": true, "database/sql": true, "math/rand": true, "io/ioutil": true,
	}
	for _, f := range scanRepo(t) {
		if !inPackage(f, "internal/domain") {
			continue
		}
		for _, imp := range f.imports {
			if forbidden[imp] || strings.HasPrefix(imp, "net") || strings.HasPrefix(imp, "os/") {
				t.Errorf("%s imports %q: the domain must stay free of IO and side effects", f.relDir, imp)
			}
		}
		assertNoWallClockCalls(t, f)
	}
}

// assertNoWallClockCalls forbids reading the wall clock in the domain. The time type is allowed for
// carrying timestamps, but time.Now/Since/Until must be injected via the Clock instead.
func assertNoWallClockCalls(t *testing.T, f goFile) {
	t.Helper()
	banned := map[string]bool{"Now": true, "Since": true, "Until": true}
	ast.Inspect(f.ast, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if ok && ident.Name == "time" && banned[sel.Sel.Name] {
			pos := f.fset.Position(sel.Pos())
			t.Errorf("%s:%d calls time.%s: read the clock via the injected Clock, not directly",
				f.relDir, pos.Line, sel.Sel.Name)
		}
		return true
	})
}

func TestApplicationDoesNotImportInfrastructure(t *testing.T) {
	for _, f := range scanRepo(t) {
		if !inPackage(f, "internal/application") {
			continue
		}
		for _, imp := range f.imports {
			if strings.HasPrefix(imp, infrastructurePkg) {
				t.Errorf("%s imports %q: the application layer must not depend on infrastructure", f.relDir, imp)
			}
			if strings.Contains(imp, wailsImportFragment) {
				t.Errorf("%s imports Wails %q: the application layer must not depend on the UI framework", f.relDir, imp)
			}
		}
	}
}

func TestNoFileExceedsLineLimit(t *testing.T) {
	for _, f := range scanRepo(t) {
		if f.lines > maxLinesPerFile {
			t.Errorf("%s has %d lines (limit %d): decompose into smaller modules", f.relDir, f.lines, maxLinesPerFile)
		}
	}
}

func TestCompositionRootIsWhitelisted(t *testing.T) {
	for _, f := range scanRepo(t) {
		importsApp := false
		importsInfra := false
		for _, imp := range f.imports {
			if imp == applicationPkg || strings.HasPrefix(imp, applicationPkg+"/") {
				importsApp = true
			}
			if strings.HasPrefix(imp, infrastructurePkg) {
				importsInfra = true
			}
		}
		if !(importsApp && importsInfra) {
			continue
		}
		// Only the composition root (package main at the repo root) may wire both layers together.
		if f.relDir != "." || f.pkg != "main" {
			t.Errorf("%s (package %s) imports both application and infrastructure: only the composition root may do this", f.relDir, f.pkg)
		}
	}
}
