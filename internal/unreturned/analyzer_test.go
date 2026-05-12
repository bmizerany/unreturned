package unreturned

import (
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"iter"
	"os"
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
	pkgs, err := testPackages(fset, dir)
	if err != nil {
		t.Fatal(err)
	}
	for name, files := range pkgs {
		t.Run(name, func(t *testing.T) {
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

func TestSourceFastPath(t *testing.T) {
	fset := token.NewFileSet()
	dir := filepath.Join("testdata", "src", "unreturnedcases")
	pkgs, err := testPackages(fset, dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, pkgFiles := range pkgs {
		files = append(files, pkgFiles...)
	}

	expected := expectedFailures(t, fset, files)
	diags, err := sourceDiagnostics([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(diags))
	for _, diag := range diags {
		got = append(got, filepath.Base(diag.Filename)+":"+strconv.Itoa(diag.Line))
	}
	slices.Sort(expected)
	slices.Sort(got)
	if !slices.Equal(got, expected) {
		t.Fatalf("diagnostics:\n got: %v\nwant: %v", got, expected)
	}
}

func TestDiagnosticText(t *testing.T) {
	pass, _ := checkedFunctions(t, `package p

func byBreak(xs []int) int {
	var picked int
	for _, x := range xs {
		picked = x
		break
	}
	return picked
}

func byGoto(xs []int) int {
	var picked int
	i := 0
again:
	if i >= len(xs) {
		goto done
	}
	if xs[i] == 0 {
		i++
		goto again
	}
	picked = xs[i]
	goto done
done:
	return picked
}
`)
	var got []string
	pass.Report = func(diag analysis.Diagnostic) {
		got = append(got, diag.Message)
	}
	if _, err := Analyzer.Run(pass); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"loop produces picked; extract as a function and return",
		"jump-loop produces picked; extract as a function and return",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("diagnostics:\n got: %v\nwant: %v", got, want)
	}
}

func TestAssignmentSequencesStop(t *testing.T) {
	pass, funcs := checkedFunctions(t, `package p

func direct(xs []int) int {
	var a, b int
	for _, x := range xs {
		a, b = x, x
		break
	}
	return a + b
}

func nested(xs []int) int {
	var a int
	for _, x := range xs {
		if x > 0 {
			a = x
			break
		}
	}
	return a
}
`)

	directState := functionState{
		pass:     pass,
		localDef: localDefs(pass.TypesInfo, funcs["direct"].Body),
	}
	directLoop := onlyRangeLoop(t, funcs["direct"])
	assign, ok := directLoop.Body.List[0].(*ast.AssignStmt)
	if !ok {
		t.Fatalf("direct loop starts with %T, want *ast.AssignStmt", directLoop.Body.List[0])
	}

	assignment, ok := firstValue(directState.assignStmtAssignments(assign, directLoop.For))
	if !ok {
		t.Fatal("assignStmtAssignments yielded no assignments")
	}
	if assignment.name != "a" {
		t.Fatalf("first assignment is %q, want a", assignment.name)
	}

	nestedState := functionState{
		pass:     pass,
		localDef: localDefs(pass.TypesInfo, funcs["nested"].Body),
	}
	nestedLoop := onlyRangeLoop(t, funcs["nested"])
	assignment, ok = firstValue(nestedState.assignments(nestedLoop.Body.List, nestedLoop.For, exitBreak, ""))
	if !ok {
		t.Fatal("assignments yielded no assignments")
	}
	if assignment.name != "a" {
		t.Fatalf("nested assignment is %q, want a", assignment.name)
	}
}

func TestNestedBlocksStops(t *testing.T) {
	stmt := &ast.IfStmt{
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.ExprStmt{X: ast.NewIdent("body")},
		}},
		Else: &ast.IfStmt{
			Body: &ast.BlockStmt{List: []ast.Stmt{
				&ast.ExprStmt{X: ast.NewIdent("elseIf")},
			}},
		},
	}

	got := countValues(nestedBlocks(stmt), 1)
	if got != 1 {
		t.Fatalf("nestedBlocks yielded %d blocks before stop, want 1", got)
	}

	got = countValues(nestedBlocks(stmt), 2)
	if got != 2 {
		t.Fatalf("nestedBlocks yielded %d blocks before else-if stop, want 2", got)
	}
}

func TestIsDirectExitRejectsUnknownExit(t *testing.T) {
	stmt := &ast.BranchStmt{Tok: token.BREAK}
	if isDirectExit(stmt, loopExit(99), "") {
		t.Fatal("isDirectExit accepted an unknown exit kind")
	}
}

func testPackages(fset *token.FileSet, dir string) (map[string][]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	pkgs := make(map[string][]*ast.File)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		ok, err := build.Default.MatchFile(dir, entry.Name())
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		pkgs[file.Name.Name] = append(pkgs[file.Name.Name], file)
	}
	for _, files := range pkgs {
		slices.SortFunc(files, func(a, b *ast.File) int {
			return strings.Compare(fset.Position(a.Package).Filename, fset.Position(b.Package).Filename)
		})
	}
	return pkgs, nil
}

func firstValue[T any](seq iter.Seq[T]) (T, bool) {
	for v := range seq {
		return v, true
	}
	var zero T
	return zero, false
}

func countValues[T any](seq iter.Seq[T], limit int) int {
	count := 0
	for range seq {
		count++
		if count == limit {
			return count
		}
	}
	return count
}

func checkedFunctions(t *testing.T, src string) (*analysis.Pass, map[string]*ast.FuncDecl) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}
	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("p", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	funcs := make(map[string]*ast.FuncDecl)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok {
			funcs[fn.Name.Name] = fn
		}
	}
	pass := &analysis.Pass{
		Analyzer:  Analyzer,
		Fset:      fset,
		Files:     []*ast.File{file},
		Pkg:       pkg,
		TypesInfo: info,
	}
	return pass, funcs
}

func onlyRangeLoop(t *testing.T, fn *ast.FuncDecl) *ast.RangeStmt {
	t.Helper()
	var loop *ast.RangeStmt
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if loop != nil {
			return false
		}
		if stmt, ok := n.(*ast.RangeStmt); ok {
			loop = stmt
			return false
		}
		return true
	})
	if loop == nil {
		t.Fatalf("%s has no range loop", fn.Name.Name)
	}
	return loop
}

func expectedFailures(t *testing.T, fset *token.FileSet, files []*ast.File) []string {
	t.Helper()
	return slices.Collect(func(yield func(string) bool) {
		for _, file := range files {
			for _, group := range file.Comments {
				for _, comment := range group.List {
					text, _ := strings.CutPrefix(comment.Text, "//")
					text = strings.TrimSpace(text)
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
