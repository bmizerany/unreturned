package unreturned

import (
	"go/ast"
	"go/token"
	"go/types"
	"iter"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// Analyzer reports loops that produce a value — extract as a function and return.
var Analyzer = &analysis.Analyzer{
	Name: "unreturned",
	Doc:  "reports loops that produce a value; extract as a function and return",
	Run:  run,
}

var fullPathFlag bool

func init() {
	Analyzer.Flags.BoolVar(&fullPathFlag, "fullpath", false, "print full file paths in diagnostics")
}

type functionState struct {
	pass     *analysis.Pass
	localDef map[types.Object]bool
}

type assignment struct {
	obj  types.Object
	name string
}

type loopExit int

const (
	exitBreak loopExit = iota
	exitGoto
)

type accessKind uint8

const (
	noAccess accessKind = iota
	readAccess
	writeAccess
)

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			state := functionState{
				pass:     pass,
				localDef: localDefs(pass.TypesInfo, fn.Body),
			}
			state.inspectBlock(fn.Body)
		}
	}
	return nil, nil
}

func localDefs(info *types.Info, body *ast.BlockStmt) map[types.Object]bool {
	defs := make(map[types.Object]bool)
	for n := range preorder(body) {
		id, ok := n.(*ast.Ident)
		if !ok {
			continue
		}
		obj, ok := info.Defs[id].(*types.Var)
		if !ok || obj.Pkg() != nil && obj.Parent() == obj.Pkg().Scope() {
			continue
		}
		defs[obj] = true
	}
	return defs
}

func (s functionState) inspectBlock(block *ast.BlockStmt) {
	for i, stmt := range block.List {
		switch stmt := stmt.(type) {
		case *ast.ForStmt:
			s.reportLoop(stmt.For, stmt.Body, block.List[i+1:])
			s.inspectBlock(stmt.Body)
		case *ast.RangeStmt:
			s.reportLoop(stmt.For, stmt.Body, block.List[i+1:])
			s.inspectBlock(stmt.Body)
		case *ast.LabeledStmt:
			if end := s.jumpLoopEnd(block.List, i, stmt.Label.Name); end >= i {
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

func (s functionState) inspectStmt(stmt ast.Stmt) {
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

func unlabeledStmt(stmt ast.Stmt) ast.Stmt {
	for {
		label, ok := stmt.(*ast.LabeledStmt)
		if !ok {
			return stmt
		}
		stmt = label.Stmt
	}
}

func (s functionState) reportLoop(pos token.Pos, body *ast.BlockStmt, after []ast.Stmt) {
	assignments := s.assignmentsIn(body.List, pos, exitBreak, "")
	if len(assignments) == 0 {
		return
	}
	if name, ok := s.readAfter(assignments, after); ok {
		s.pass.Reportf(pos, "loop produces %s; extract as a function and return", name)
	}
}

func (s functionState) reportJumpLoop(pos token.Pos, label string, body, after []ast.Stmt) {
	assignments := s.assignmentsIn(body, pos, exitGoto, label)
	if len(assignments) == 0 {
		return
	}
	if name, ok := s.readAfter(assignments, after); ok {
		s.pass.Reportf(pos, "jump-loop produces %s; extract as a function and return", name)
	}
}

func (s functionState) assignmentsIn(stmts []ast.Stmt, loopPos token.Pos, exit loopExit, loopLabel string) []assignment {
	return slices.Collect(s.assignments(stmts, loopPos, exit, loopLabel))
}

func (s functionState) assignments(stmts []ast.Stmt, loopPos token.Pos, exit loopExit, loopLabel string) iter.Seq[assignment] {
	return func(yield func(assignment) bool) {
		seen := make(map[types.Object]bool)
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

func (s functionState) directAssignments(stmt ast.Stmt, loopPos token.Pos) iter.Seq[assignment] {
	return func(yield func(assignment) bool) {
		switch stmt := stmt.(type) {
		case *ast.AssignStmt:
			s.assignStmtAssignments(stmt, loopPos)(yield)
		case *ast.IncDecStmt:
			id, ok := stmt.X.(*ast.Ident)
			if !ok {
				return
			}
			obj := s.pass.TypesInfo.Uses[id]
			if obj == nil || !s.isOuterLocal(obj, loopPos) {
				return
			}
			yield(assignment{obj: obj, name: id.Name})
		}
	}
}

func (s functionState) assignStmtAssignments(stmt *ast.AssignStmt, loopPos token.Pos) iter.Seq[assignment] {
	return func(yield func(assignment) bool) {
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
			if !yield(assignment{obj: obj, name: id.Name}) {
				return
			}
		}
	}
}

func (s functionState) assignedObject(tok token.Token, id *ast.Ident) types.Object {
	if id.Name == "_" {
		return nil
	}
	if tok == token.DEFINE && s.pass.TypesInfo.Defs[id] != nil {
		return nil
	}
	return s.pass.TypesInfo.Uses[id]
}

func (s functionState) isBuiltinAppend(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return false
	}
	obj, ok := s.pass.TypesInfo.Uses[id].(*types.Builtin)
	return ok && obj.Name() == "append"
}

func (s functionState) isOuterLocal(obj types.Object, pos token.Pos) bool {
	v, ok := obj.(*types.Var)
	if !ok || v.IsField() || !s.localDef[obj] {
		return false
	}
	return obj.Pos().IsValid() && obj.Pos() < pos
}

func (s functionState) hasExitBeforeContinue(stmts []ast.Stmt, exit loopExit, loopLabel string) bool {
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

func isDirectExit(stmt ast.Stmt, exit loopExit, loopLabel string) bool {
	branch, ok := stmt.(*ast.BranchStmt)
	if !ok {
		return false
	}
	switch exit {
	case exitBreak:
		return branch.Tok == token.BREAK
	case exitGoto:
		return branch.Tok == token.GOTO && branch.Label != nil && branch.Label.Name != loopLabel
	default:
		return false
	}
}

func isDirectStop(stmt ast.Stmt) bool {
	branch, ok := stmt.(*ast.BranchStmt)
	if !ok {
		return false
	}
	return branch.Tok == token.BREAK || branch.Tok == token.CONTINUE || branch.Tok == token.GOTO || branch.Tok == token.FALLTHROUGH
}

func hasContinue(stmt ast.Stmt) bool {
	switch stmt.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		return false
	}
	found := false
	root := ast.Node(stmt)
	ast.Inspect(stmt, func(n ast.Node) bool {
		if n == nil || found {
			return true
		}
		if n != root {
			switch n.(type) {
			case *ast.FuncLit, *ast.ForStmt, *ast.RangeStmt:
				return false
			}
		}
		branch, ok := n.(*ast.BranchStmt)
		if ok && branch.Tok == token.CONTINUE {
			found = true
		}
		return true
	})
	return found
}

func nestedBlocks(stmt ast.Stmt) iter.Seq[[]ast.Stmt] {
	return func(yield func([]ast.Stmt) bool) {
		switch stmt := stmt.(type) {
		case *ast.BlockStmt:
			yield(stmt.List)
		case *ast.IfStmt:
			for {
				if !yield(stmt.Body.List) {
					return
				}
				switch els := stmt.Else.(type) {
				case *ast.BlockStmt:
					yield(els.List)
					return
				case *ast.IfStmt:
					stmt = els
				default:
					return
				}
			}
		case *ast.LabeledStmt:
			if block, ok := stmt.Stmt.(*ast.BlockStmt); ok {
				yield(block.List)
			}
		}
	}
}

func (s functionState) readAfter(assignments []assignment, stmts []ast.Stmt) (string, bool) {
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

func (s functionState) accessIn(node ast.Node, target types.Object) accessKind {
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
				if id, ok := lhs.(*ast.Ident); ok && s.pass.TypesInfo.Uses[id] == target {
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
			if id, ok := n.X.(*ast.Ident); ok && s.pass.TypesInfo.Uses[id] == target {
				read = true
				return true
			}
		case *ast.RangeStmt:
			if s.readsObject(n.X, target) {
				read = true
				return true
			}
			if rangeAssignsObject(s.pass.TypesInfo, n, target) {
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
			if s.pass.TypesInfo.Uses[n] == target {
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

func (s functionState) readsObject(node ast.Node, target types.Object) bool {
	for n := range preorder(node) {
		id, ok := n.(*ast.Ident)
		if !ok {
			continue
		}
		if s.pass.TypesInfo.Uses[id] == target {
			return true
		}
	}
	return false
}

func rangeAssignsObject(info *types.Info, stmt *ast.RangeStmt, target types.Object) bool {
	for _, expr := range []ast.Expr{stmt.Key, stmt.Value} {
		id, ok := expr.(*ast.Ident)
		if !ok {
			continue
		}
		if info.Uses[id] == target {
			return true
		}
	}
	return false
}

func (s functionState) jumpLoopEnd(stmts []ast.Stmt, labelIndex int, label string) int {
	labelPos := stmts[labelIndex].Pos()
	backward := false
	for offset, stmt := range stmts[labelIndex+1:] {
		i := labelIndex + 1 + offset
		if _, ok := stmt.(*ast.LabeledStmt); ok && backward {
			return i - 1
		}
		if hasBackwardGoto(stmt, label, labelPos) {
			backward = true
		}
	}
	if backward {
		return len(stmts) - 1
	}
	return -1
}

func hasBackwardGoto(node ast.Node, label string, labelPos token.Pos) bool {
	for n := range preorder(node) {
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

func preorder(root ast.Node) iter.Seq[ast.Node] {
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
