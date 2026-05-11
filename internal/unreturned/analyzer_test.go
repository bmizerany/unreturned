package unreturned

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestAnalyzer(t *testing.T) {
	fset := token.NewFileSet()
	dir := filepath.Join("testdata", "src", "unreturnedcases")
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for name, astpkg := range pkgs {
		t.Run(name, func(t *testing.T) {
			files := packageFiles(fset, astpkg)
			info := &types.Info{
				Types:      make(map[ast.Expr]types.TypeAndValue),
				Defs:       make(map[*ast.Ident]types.Object),
				Uses:       make(map[*ast.Ident]types.Object),
				Implicits:  make(map[ast.Node]types.Object),
				Selections: make(map[*ast.SelectorExpr]*types.Selection),
				Scopes:     make(map[ast.Node]*types.Scope),
			}
			conf := types.Config{Importer: importer.Default()}
			pkg, err := conf.Check("blake.io/unreturned/internal/unreturned/testdata/src/"+name, fset, files, info)
			if err != nil {
				t.Fatal(err)
			}

			expected := expectedFailures(t, fset, files)
			var got []string
			pass := &analysis.Pass{
				Analyzer:  Analyzer,
				Fset:      fset,
				Files:     files,
				Pkg:       pkg,
				TypesInfo: info,
				Report: func(diag analysis.Diagnostic) {
					pos := fset.Position(diag.Pos)
					got = append(got, filepath.Base(pos.Filename)+":"+strconv.Itoa(pos.Line))
				},
			}
			if _, err := Analyzer.Run(pass); err != nil {
				t.Fatal(err)
			}
			slices.Sort(expected)
			slices.Sort(got)
			if !slices.Equal(got, expected) {
				t.Fatalf("diagnostics:\n got: %v\nwant: %v", got, expected)
			}
		})
	}
}

func packageFiles(fset *token.FileSet, pkg *ast.Package) []*ast.File {
	files := slices.Collect(maps.Values(pkg.Files))
	slices.SortFunc(files, func(a, b *ast.File) int {
		return strings.Compare(fset.Position(a.Package).Filename, fset.Position(b.Package).Filename)
	})
	return files
}

func expectedFailures(t *testing.T, fset *token.FileSet, files []*ast.File) []string {
	t.Helper()
	return slices.Collect(func(yield func(string) bool) {
		for _, file := range files {
			for _, group := range file.Comments {
				for _, comment := range group.List {
					text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
					if text != "Unreturned: Fail" {
						continue
					}
					pos := fset.Position(comment.Pos())
					if !yield(filepath.Base(pos.Filename) + ":" + strconv.Itoa(pos.Line+1)) {
						return
					}
				}
			}
		}
	})
}
