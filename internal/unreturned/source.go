package unreturned

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"iter"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

const diagnosticsExit = 3

var (
	breakBytes = []byte("break")
	gotoBytes  = []byte("goto")
)

// CanRunSource reports whether args are plain local package patterns that can
// be checked directly from source files.
func CanRunSource(args []string) bool {
	if len(args) == 0 {
		return false
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") || strings.HasSuffix(arg, ".cfg") || strings.HasSuffix(arg, ".go") {
			return false
		}
		if !isLocalPattern(arg) {
			return false
		}
	}
	return true
}

func isLocalPattern(arg string) bool {
	return arg == "." || strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "../") || filepath.IsAbs(arg)
}

// RunSource checks args and writes diagnostics in the usual vet form.
func RunSource(w io.Writer, args []string) (int, error) {
	diags, err := sourceDiagnostics(args)
	if err != nil {
		return 0, err
	}
	for _, diag := range diags {
		fmt.Fprintf(w, "%s:%d:%d: %s\n", diag.Filename, diag.Line, diag.Column, diag.Message)
	}
	if len(diags) > 0 {
		return diagnosticsExit, nil
	}
	return 0, nil
}

type sourceDiagnostic struct {
	Filename string
	Line     int
	Column   int
	Message  string
}

type sourceInput struct {
	name string
	src  []byte
}

// sourceDiagnostics checks local package patterns without loading or
// type-checking packages.
func sourceDiagnostics(args []string) ([]sourceDiagnostic, error) {
	files, err := sourceFiles(args)
	if err != nil {
		return nil, err
	}

	var diags []sourceDiagnostic
	var mu sync.Mutex
	var firstErr error
	jobs := make(chan sourceInput)
	workers := min(runtime.GOMAXPROCS(0), len(files))
	if workers == 0 {
		return nil, nil
	}

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for file := range jobs {
				got, err := fileDiagnostics(file)
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
				}
				diags = append(diags, got...)
				mu.Unlock()
			}
		})
	}
	for _, file := range files {
		jobs <- file
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	slices.SortFunc(diags, cmpDiag)
	return diags, nil
}

func cmpDiag(a, b sourceDiagnostic) int {
	if n := strings.Compare(a.Filename, b.Filename); n != 0 {
		return n
	}
	if a.Line != b.Line {
		return a.Line - b.Line
	}
	if a.Column != b.Column {
		return a.Column - b.Column
	}
	return strings.Compare(a.Message, b.Message)
}

func sourceFiles(args []string) ([]sourceInput, error) {
	seen := make(map[string]bool)
	var files []sourceInput
	for _, arg := range args {
		got, err := patternFiles(arg)
		if err != nil {
			return nil, err
		}
		for _, file := range got {
			if seen[file.name] {
				continue
			}
			seen[file.name] = true
			files = append(files, file)
		}
	}
	slices.SortFunc(files, func(a, b sourceInput) int {
		return strings.Compare(a.name, b.name)
	})
	return files, nil
}

func patternFiles(pattern string) ([]sourceInput, error) {
	if strings.HasSuffix(pattern, "/...") {
		root := strings.TrimSuffix(pattern, "/...")
		if root == "" {
			root = "."
		}
		return treeFiles(root)
	}
	return dirFiles(pattern)
}

func treeFiles(root string) ([]sourceInput, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && ignoredDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isGoFile(d.Name()) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return candidateFiles(paths)
}

func dirFiles(dir string) ([]sourceInput, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !isGoFile(entry.Name()) {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	return candidateFiles(files)
}

type candidateResult struct {
	file sourceInput
	ok   bool
	err  error
}

func candidateFiles(paths []string) ([]sourceInput, error) {
	workers := min(runtime.GOMAXPROCS(0), len(paths))
	if workers == 0 {
		return nil, nil
	}

	jobs := make(chan string)
	results := make(chan candidateResult, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for path := range jobs {
				file, ok, err := candidateFile(path)
				results <- candidateResult{file: file, ok: ok, err: err}
			}
		})
	}
	go func() {
		for _, path := range paths {
			jobs <- path
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	var files []sourceInput
	var firstErr error
	for result := range results {
		if result.err != nil && firstErr == nil {
			firstErr = result.err
		}
		if result.ok {
			files = append(files, result.file)
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return files, nil
}

func candidateFile(file string) (sourceInput, bool, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return sourceInput{}, false, err
	}
	if !bytes.Contains(src, breakBytes) && !bytes.Contains(src, gotoBytes) {
		return sourceInput{}, false, nil
	}
	ok, err := build.Default.MatchFile(filepath.Dir(file), filepath.Base(file))
	if err != nil {
		return sourceInput{}, false, err
	}
	if !ok {
		return sourceInput{}, false, nil
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return sourceInput{}, false, err
	}
	return sourceInput{name: abs, src: src}, true, nil
}

func ignoredDir(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func isGoFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "_")
}

func fileDiagnostics(file sourceInput) ([]sourceDiagnostic, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file.name, file.src, 0)
	if err != nil {
		return nil, err
	}
	state := sourceFileState{fset: fset}
	return state.diagnostics(parsed), nil
}

type sourceFileState struct {
	fset  *token.FileSet
	diags []sourceDiagnostic
}

type sourceFunctionState struct {
	file     *sourceFileState
	localDef map[*ast.Object]bool
}

type sourceAssignment struct {
	obj  *ast.Object
	name string
}

func (s *sourceFileState) diagnostics(file *ast.File) []sourceDiagnostic {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		state := sourceFunctionState{
			file:     s,
			localDef: sourceLocalDefs(fn.Body),
		}
		state.inspectBlock(fn.Body)
	}
	return s.diags
}

func sourceLocalDefs(body *ast.BlockStmt) map[*ast.Object]bool {
	defs := make(map[*ast.Object]bool)
	for n := range sourcePreorder(body) {
		id, ok := n.(*ast.Ident)
		if !ok || id.Obj == nil || id.Obj.Kind != ast.Var || id.Obj.Pos() != id.Pos() {
			continue
		}
		defs[id.Obj] = true
	}
	return defs
}

func (s sourceFunctionState) inspectBlock(block *ast.BlockStmt) {
	for i, stmt := range block.List {
		switch stmt := stmt.(type) {
		case *ast.ForStmt:
			s.reportLoop(stmt.For, stmt.Body, block.List[i+1:])
			s.inspectBlock(stmt.Body)
		case *ast.RangeStmt:
			s.reportLoop(stmt.For, stmt.Body, block.List[i+1:])
			s.inspectBlock(stmt.Body)
		case *ast.LabeledStmt:
			if end := sourceJumpLoopEnd(block.List, i, stmt.Label.Name); end >= i {
				s.reportJumpLoop(stmt.Pos(), stmt.Label.Name, block.List[i:end+1], block.List[end+1:])
			}
			switch stmt := unlabeledStmt(stmt.Stmt).(type) {
			case *ast.ForStmt:
				s.reportLoop(stmt.For, stmt.Body, block.List[i+1:])
				s.inspectBlock(stmt.Body)
			case *ast.RangeStmt:
				s.reportLoop(stmt.For, stmt.Body, block.List[i+1:])
				s.inspectBlock(stmt.Body)
			default:
				s.inspectStmt(stmt)
			}
		default:
			s.inspectStmt(stmt)
		}
	}
}

func (s sourceFunctionState) inspectStmt(stmt ast.Stmt) {
	switch stmt := stmt.(type) {
	case *ast.BlockStmt:
		s.inspectBlock(stmt)
	case *ast.IfStmt:
		if stmt.Init != nil {
			s.inspectStmt(stmt.Init)
		}
		s.inspectBlock(stmt.Body)
		switch els := stmt.Else.(type) {
		case *ast.BlockStmt:
			s.inspectBlock(els)
		case *ast.IfStmt:
			s.inspectStmt(els)
		}
	case *ast.SwitchStmt:
		if stmt.Init != nil {
			s.inspectStmt(stmt.Init)
		}
		for _, clause := range stmt.Body.List {
			clause := clause.(*ast.CaseClause)
			s.inspectBlock(&ast.BlockStmt{List: clause.Body})
		}
	case *ast.TypeSwitchStmt:
		if stmt.Init != nil {
			s.inspectStmt(stmt.Init)
		}
		if stmt.Assign != nil {
			s.inspectStmt(stmt.Assign)
		}
		for _, clause := range stmt.Body.List {
			clause := clause.(*ast.CaseClause)
			s.inspectBlock(&ast.BlockStmt{List: clause.Body})
		}
	case *ast.SelectStmt:
		for _, clause := range stmt.Body.List {
			clause := clause.(*ast.CommClause)
			if clause.Comm != nil {
				s.inspectStmt(clause.Comm)
			}
			s.inspectBlock(&ast.BlockStmt{List: clause.Body})
		}
	}
}

func (s sourceFunctionState) reportLoop(pos token.Pos, body *ast.BlockStmt, after []ast.Stmt) {
	assignments := s.assignmentsIn(body.List, pos, exitBreak, "")
	if len(assignments) == 0 {
		return
	}
	if name, ok := s.readAfter(assignments, after); ok {
		s.report(pos, "loop produces "+name+"; extract as a function and return")
	}
}

func (s sourceFunctionState) reportJumpLoop(pos token.Pos, label string, body, after []ast.Stmt) {
	assignments := s.assignmentsIn(body, pos, exitGoto, label)
	if len(assignments) == 0 {
		return
	}
	if name, ok := s.readAfter(assignments, after); ok {
		s.report(pos, "jump-loop produces "+name+"; extract as a function and return")
	}
}

func (s sourceFunctionState) report(pos token.Pos, msg string) {
	p := s.file.fset.Position(pos)
	s.file.diags = append(s.file.diags, sourceDiagnostic{
		Filename: p.Filename,
		Line:     p.Line,
		Column:   p.Column,
		Message:  msg,
	})
}

func (s sourceFunctionState) assignmentsIn(stmts []ast.Stmt, loopPos token.Pos, exit loopExit, loopLabel string) []sourceAssignment {
	return slices.Collect(s.assignments(stmts, loopPos, exit, loopLabel))
}

func (s sourceFunctionState) assignments(stmts []ast.Stmt, loopPos token.Pos, exit loopExit, loopLabel string) iter.Seq[sourceAssignment] {
	return func(yield func(sourceAssignment) bool) {
		seen := make(map[*ast.Object]bool)
		var walk func([]ast.Stmt) bool
		walk = func(stmts []ast.Stmt) bool {
			for i, stmt := range stmts {
				if s.hasExitBeforeContinue(stmts[i+1:], exit, loopLabel) {
					for assignment := range s.directAssignments(stmt, loopPos) {
						if seen[assignment.obj] {
							continue
						}
						seen[assignment.obj] = true
						if !yield(assignment) {
							return false
						}
					}
				}
				for block := range nestedBlocks(stmt) {
					if !walk(block) {
						return false
					}
				}
			}
			return true
		}
		walk(stmts)
	}
}

func (s sourceFunctionState) directAssignments(stmt ast.Stmt, loopPos token.Pos) iter.Seq[sourceAssignment] {
	return func(yield func(sourceAssignment) bool) {
		switch stmt := stmt.(type) {
		case *ast.AssignStmt:
			s.assignStmtAssignments(stmt, loopPos)(yield)
		case *ast.IncDecStmt:
			id, ok := stmt.X.(*ast.Ident)
			if !ok {
				return
			}
			obj := id.Obj
			if obj == nil || !s.isOuterLocal(obj, loopPos) {
				return
			}
			yield(sourceAssignment{obj: obj, name: id.Name})
		}
	}
}

func (s sourceFunctionState) assignStmtAssignments(stmt *ast.AssignStmt, loopPos token.Pos) iter.Seq[sourceAssignment] {
	return func(yield func(sourceAssignment) bool) {
		for i, lhs := range stmt.Lhs {
			id, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}
			obj := s.assignedObject(stmt.Tok, id)
			if obj == nil || !s.isOuterLocal(obj, loopPos) {
				continue
			}
			if i < len(stmt.Rhs) && s.isBuiltinAppend(stmt.Rhs[i]) {
				continue
			}
			if !yield(sourceAssignment{obj: obj, name: id.Name}) {
				return
			}
		}
	}
}

func (s sourceFunctionState) assignedObject(tok token.Token, id *ast.Ident) *ast.Object {
	if id.Name == "_" {
		return nil
	}
	if tok == token.DEFINE && id.Obj != nil && id.Obj.Pos() == id.Pos() {
		return nil
	}
	return id.Obj
}

func (s sourceFunctionState) isBuiltinAppend(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	return ok && id.Name == "append" && id.Obj == nil
}

func (s sourceFunctionState) isOuterLocal(obj *ast.Object, pos token.Pos) bool {
	return obj != nil && obj.Kind == ast.Var && s.localDef[obj] && obj.Pos().IsValid() && obj.Pos() < pos
}

func (s sourceFunctionState) hasExitBeforeContinue(stmts []ast.Stmt, exit loopExit, loopLabel string) bool {
	for _, stmt := range stmts {
		if hasContinue(stmt) {
			return false
		}
		if isDirectExit(stmt, exit, loopLabel) {
			return true
		}
		if isDirectStop(stmt) {
			return false
		}
	}
	return false
}

func (s sourceFunctionState) readAfter(assignments []sourceAssignment, stmts []ast.Stmt) (string, bool) {
	live := slices.Clone(assignments)
	for _, stmt := range stmts {
		for i := 0; i < len(live); {
			access := s.accessIn(stmt, live[i].obj)
			switch access {
			case readAccess:
				return live[i].name, true
			case writeAccess:
				live = slices.Delete(live, i, i+1)
			default:
				i++
			}
		}
	}
	return "", false
}

func (s sourceFunctionState) accessIn(node ast.Node, target *ast.Object) accessKind {
	var read, write bool
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil || read {
			return true
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, rhs := range n.Rhs {
				if s.readsObject(rhs, target) {
					read = true
					return true
				}
			}
			for _, lhs := range n.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Obj == target {
					if n.Tok == token.ASSIGN || n.Tok == token.DEFINE {
						write = true
						continue
					}
					read = true
					return true
				}
				if s.readsObject(lhs, target) {
					read = true
					return true
				}
			}
			return false
		case *ast.IncDecStmt:
			if id, ok := n.X.(*ast.Ident); ok && id.Obj == target {
				read = true
				return true
			}
		case *ast.RangeStmt:
			if s.readsObject(n.X, target) {
				read = true
				return true
			}
			if sourceRangeAssignsObject(n, target) {
				write = true
				return false
			}
			switch s.accessIn(n.Body, target) {
			case readAccess:
				read = true
			case writeAccess:
				write = true
			}
			return false
		case *ast.Ident:
			if n.Obj == target {
				read = true
				return true
			}
		}
		return true
	})
	if read {
		return readAccess
	}
	if write {
		return writeAccess
	}
	return noAccess
}

func (s sourceFunctionState) readsObject(node ast.Node, target *ast.Object) bool {
	for n := range sourcePreorder(node) {
		id, ok := n.(*ast.Ident)
		if !ok {
			continue
		}
		if id.Obj == target {
			return true
		}
	}
	return false
}

func sourceRangeAssignsObject(stmt *ast.RangeStmt, target *ast.Object) bool {
	for _, expr := range []ast.Expr{stmt.Key, stmt.Value} {
		id, ok := expr.(*ast.Ident)
		if !ok {
			continue
		}
		if id.Obj == target {
			return true
		}
	}
	return false
}

func sourceJumpLoopEnd(stmts []ast.Stmt, labelIndex int, label string) int {
	labelPos := stmts[labelIndex].Pos()
	backward := false
	for offset, stmt := range stmts[labelIndex+1:] {
		i := labelIndex + 1 + offset
		if _, ok := stmt.(*ast.LabeledStmt); ok && backward {
			return i - 1
		}
		if sourceHasBackwardGoto(stmt, label, labelPos) {
			backward = true
		}
	}
	if backward {
		return len(stmts) - 1
	}
	return -1
}

func sourceHasBackwardGoto(node ast.Node, label string, labelPos token.Pos) bool {
	for n := range sourcePreorder(node) {
		branch, ok := n.(*ast.BranchStmt)
		if !ok || branch.Tok != token.GOTO || branch.Label == nil {
			continue
		}
		if branch.Label.Name == label && branch.Pos() > labelPos {
			return true
		}
	}
	return false
}

func sourcePreorder(root ast.Node) iter.Seq[ast.Node] {
	return func(yield func(ast.Node) bool) {
		stopped := false
		ast.PreorderStack(root, nil, func(n ast.Node, _ []ast.Node) bool {
			if stopped {
				return false
			}
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			stopped = !yield(n)
			return !stopped
		})
	}
}
