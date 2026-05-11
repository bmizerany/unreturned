package unreturned

import (
	"go/ast"
	"go/token"
	"go/types"
	"iter"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// Analyzer reports loop-assigned temporary variables read after the loop.
var Analyzer = &analysis.Analyzer{
	Name: "unreturned",
	Doc:  "reports local variables assigned inside loops and read after the loop exits",
	Run:  run,
}

type functionState struct {
	pass     *analysis.Pass
	localDef map[types.Object]bool
}

type assignment struct {
	obj  types.Object
	name string
}

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
				s.reportJumpLoop(stmt.Pos(), block.List[i:end+1], block.List[end+1:])
			}
			switch stmt := stmt.Stmt.(type) {
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
	case *ast.ForStmt:
		s.reportLoop(stmt.For, stmt.Body, nil)
		s.inspectBlock(stmt.Body)
	case *ast.RangeStmt:
		s.reportLoop(stmt.For, stmt.Body, nil)
		s.inspectBlock(stmt.Body)
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
	case *ast.LabeledStmt:
		s.inspectStmt(stmt.Stmt)
	}
}

func (s functionState) reportLoop(pos token.Pos, body *ast.BlockStmt, after []ast.Stmt) {
	assignments := s.assignmentsIn(body.List, pos)
	if len(assignments) == 0 {
		return
	}
	if name, ok := s.readAfter(assignments, after); ok {
		s.pass.Reportf(pos, "variable %s is assigned inside a loop and read after the loop exits", name)
	}
}

func (s functionState) reportJumpLoop(pos token.Pos, body, after []ast.Stmt) {
	assignments := s.assignmentsIn(body, pos)
	if len(assignments) == 0 {
		return
	}
	if name, ok := s.readAfter(assignments, after); ok {
		s.pass.Reportf(pos, "variable %s is assigned inside a jump loop and read after the loop exits", name)
	}
}

func (s functionState) assignmentsIn(stmts []ast.Stmt, loopPos token.Pos) []assignment {
	return slices.Collect(s.assignments(stmts, loopPos))
}

func (s functionState) assignments(stmts []ast.Stmt, loopPos token.Pos) iter.Seq[assignment] {
	return func(yield func(assignment) bool) {
		seen := make(map[types.Object]bool)
		for _, stmt := range stmts {
			for n := range preorder(stmt) {
				switch n := n.(type) {
				case *ast.AssignStmt:
					for _, lhs := range n.Lhs {
						id, ok := lhs.(*ast.Ident)
						if !ok {
							continue
						}
						obj := s.assignedObject(n.Tok, id)
						if obj == nil || seen[obj] || !s.isOuterLocal(obj, loopPos) {
							continue
						}
						seen[obj] = true
						if !yield(assignment{obj: obj, name: id.Name}) {
							return
						}
					}
				case *ast.IncDecStmt:
					id, ok := n.X.(*ast.Ident)
					if !ok {
						continue
					}
					obj := s.pass.TypesInfo.Uses[id]
					if obj == nil || seen[obj] || !s.isOuterLocal(obj, loopPos) {
						continue
					}
					seen[obj] = true
					if !yield(assignment{obj: obj, name: id.Name}) {
						return
					}
				}
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

func (s functionState) isOuterLocal(obj types.Object, pos token.Pos) bool {
	v, ok := obj.(*types.Var)
	if !ok || v.IsField() || !s.localDef[obj] {
		return false
	}
	return obj.Pos().IsValid() && obj.Pos() < pos
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
	for i := labelIndex + 1; i < len(stmts); i++ {
		if hasBackwardGoto(stmts[i], label, labelPos) {
			return i
		}
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
