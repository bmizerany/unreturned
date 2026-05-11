// Code generated for unreturned analyzer tests. DO NOT EDIT.

package unreturnedcases

var packageTemp int

type namedInt int

func rememberInt(int) {}

func pickInt(x int) int { return x }

func FailRangeLoop(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		break
	}
	return picked
}

func FailThreeClauseFor(xs []string) string {
	var picked string
	// Unreturned: Fail
	for i := 0; i < len(xs); i++ {
		if xs[i] != "" {
			picked = xs[i]
			break
		}
	}
	return picked
}

func FailConditionFor(xs []int) int {
	var picked int
	i := 0
	// Unreturned: Fail
	for i < len(xs) {
		picked = xs[i]
		i++
		break
	}
	return picked
}

func FailJumpLoop(xs []int) int {
	var picked int
	i := 0
	// Unreturned: Fail
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

func FailShadowedAppend(xs []int) []int {
	var picked []int
	append := func([]int, int) []int {
		return nil
	}
	// Unreturned: Fail
	for _, x := range xs {
		picked = append(picked, x)
		break
	}
	return picked
}

func FailAssignCallBreak(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		rememberInt(picked)
		break
	}
	return picked
}

func FailLabeledFor(xs []int) int {
	var picked int
outer:
	// Unreturned: Fail
	for i := 0; i < len(xs); i++ {
		picked = xs[i]
		break outer
	}
	return picked
}

func FailLabeledRange(xs []int) int {
	var picked int
outer:
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		break outer
	}
	return picked
}

func FailDoubleLabeledFor(xs []int) int {
	var picked int
	i := 0
	if len(xs) < 0 {
		goto outer
	}
outer:
inner:
	// Unreturned: Fail
	for i < len(xs) {
		picked = xs[i]
		break inner
	}
	return picked
}

func FailDoubleLabeledRange(xs []int) int {
	var picked int
	if len(xs) < 0 {
		goto outer
	}
outer:
inner:
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		break inner
	}
	return picked
}

func FailDuplicateAssignBeforeBreak(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		picked = x + 1
		break
	}
	return picked
}

func FailIfInitLoop(xs []int) int {
	var picked int
	if ok := len(xs) > 0; ok {
		// Unreturned: Fail
		for _, x := range xs {
			picked = x
			break
		}
		return picked
	}
	return 0
}

func FailElseLoop(xs []int) int {
	var picked int
	if len(xs) == 0 {
		return 0
	} else {
		// Unreturned: Fail
		for _, x := range xs {
			picked = x
			break
		}
		return picked
	}
}

func FailElseIfLoop(xs []int) int {
	var picked int
	if len(xs) == 0 {
		return 0
	} else if len(xs) > 0 {
		// Unreturned: Fail
		for _, x := range xs {
			picked = x
			break
		}
		return picked
	}
	return 0
}

func FailSwitchLoop(xs []int) int {
	var picked int
	switch n := len(xs); {
	case n > 0:
		// Unreturned: Fail
		for _, x := range xs {
			picked = x
			break
		}
		return picked
	}
	return 0
}

func FailTypeSwitchLoop(v any, xs []int) int {
	var picked int
	switch y := v; y.(type) {
	case []int:
		// Unreturned: Fail
		for _, x := range xs {
			picked = x
			break
		}
		return picked
	}
	return 0
}

func FailSelectLoop(ch <-chan int, xs []int) int {
	var picked int
	select {
	case <-ch:
		// Unreturned: Fail
		for _, x := range xs {
			picked = x
			break
		}
		return picked
	default:
	}
	return 0
}

func FailReadAfterUnrelatedStmt(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		break
	}
	rememberInt(0)
	return picked
}

func FailReadInAssignRHS(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		break
	}
	other := picked + pickInt(0)
	return other
}

func FailReadInCompoundAssign(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		break
	}
	picked += 1
	return picked
}

func FailReadInAssignLHS(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		break
	}
	seen := map[int]bool{}
	seen[picked] = true
	return 0
}

func FailReadInIncDec(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		break
	}
	picked++
	return picked
}

func FailReadInRangeExpr(xs []int) int {
	var picked []int
	// Unreturned: Fail
	for _, x := range xs {
		picked = []int{x}
		break
	}
	for range picked {
		return 1
	}
	return 0
}

func FailReadThroughFuncLiteralIgnored(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		break
	}
	func() {
		_ = picked
	}()
	return picked
}

func FailReadAfterNestedLoopContinue(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		for range xs {
			continue
		}
		break
	}
	return picked
}

func FailRangeBodyReadAfterLoop(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		break
	}
	for range xs {
		return picked
	}
	return 0
}

func FailRangeNoAssignNoReadBody(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
		break
	}
	for range xs {
	}
	return picked
}

func FailNestedBlockAssignBreak(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		{
			picked = x
			break
		}
	}
	return picked
}

func FailNestedElseAssignBreak(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		if x < 0 {
		} else {
			picked = x
			break
		}
	}
	return picked
}

func FailNestedElseIfAssignBreak(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		if x < 0 {
		} else if x >= 0 {
			picked = x
			break
		}
	}
	return picked
}

func FailNestedLabeledBlockAssignBreak(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		if x < 0 {
			goto label
		}
	label:
		{
			picked = x
			break
		}
	}
	return picked
}

func PassNoBreakAfterAssignment(xs []int) int {
	var picked int
	for _, x := range xs {
		picked = x
	}
	return picked
}

func PassAssignThenContinue(xs []int) int {
	var picked int
	for _, x := range xs {
		picked = x
		continue
	}
	return picked
}

func PassAssignThenPossibleContinue(xs []int) int {
	var picked int
	for _, x := range xs {
		picked = x
		if x < 0 {
			continue
		}
		break
	}
	return picked
}

func PassBlockLoopReadOutsideBlock(xs []int) int {
	var picked int
	{
		for _, x := range xs {
			picked = x
			break
		}
	}
	return picked
}

func PassElseBlockTraversal(xs []int) int {
	var picked int
	if len(xs) > 0 {
		return 0
	} else {
		{
			for _, x := range xs {
				picked = x
				break
			}
		}
	}
	return picked
}

func PassElseIfTraversal(xs []int) int {
	var picked int
	if len(xs) > 0 {
		return 0
	} else if len(xs) == 0 {
		{
			for _, x := range xs {
				picked = x
				break
			}
		}
	}
	return picked
}

func PassLabeledBlockTraversal(xs []int) int {
	var picked int
	if len(xs) < 0 {
		goto label
	}
label:
	{
		for _, x := range xs {
			picked = x
			break
		}
	}
	return picked
}

func PassRangeNoReadAfter(xs []int) {
	var picked int
	for _, x := range xs {
		picked = x
		break
		_ = picked
	}
}

func PassOverwrittenBeforeRead(xs []int) int {
	var picked int
	for _, x := range xs {
		picked = x
		break
	}
	picked = 0
	return picked
}

func PassReadOnlyAfter(xs []int) int {
	count := len(xs)
	for range xs {
	}
	return count
}

func PassLocalInsideLoop(xs []int) int {
	total := len(xs)
	for _, x := range xs {
		picked := x
		_ = picked
	}
	return total
}

func PassPackageVariable(xs []int) int {
	for _, x := range xs {
		packageTemp = x
		break
	}
	return packageTemp
}

func PassLocalDefineWithBreak(xs []int) int {
	total := len(xs)
	for _, x := range xs {
		picked := x
		_ = picked
		break
	}
	return total
}

func PassBlankAssignWithBreak(xs []int) int {
	var picked int
	for _, x := range xs {
		_ = x
		break
	}
	return picked
}

func PassIndexAssignWithBreak(xs []int) []int {
	picked := []int{0}
	for _, x := range xs {
		picked[0] = x
		break
	}
	return picked
}

func PassIncDecNonIdentWithBreak(p *int) int {
	for {
		(*p)++
		break
	}
	return *p
}

func PassIncDecInnerLocalWithBreak(xs []int) int {
	var picked int
	for range xs {
		local := 0
		local++
		break
	}
	return picked
}

func FailFuncLiteralAssignWithBreak(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = func() int { return x }()
		break
	}
	return picked
}

func FailTypeConversionAssignWithBreak(xs []int) namedInt {
	var picked namedInt
	// Unreturned: Fail
	for _, x := range xs {
		picked = namedInt(x)
		break
	}
	return picked
}

func PassRangeAssignOverwritesBeforeRead(xs []int) int {
	var picked int
	for _, x := range xs {
		picked = x
		break
	}
	for picked = range xs {
		return picked
	}
	return picked
}

func PassRangeBodyWriteOverwritesBeforeRead(xs []int) int {
	var picked int
	for _, x := range xs {
		picked = x
		break
	}
	for range xs {
		picked = 0
	}
	return picked
}

func PassNestedContinueInsideIfBeforeBreak(xs []int) int {
	var picked int
	for _, x := range xs {
		picked = x
		if x >= 0 {
			for range xs {
				continue
			}
		}
		if x < 0 {
			continue
		}
		break
	}
	return picked
}

func PassJumpLoopNoAssignment(xs []int) int {
	i := 0
again:
	if i >= len(xs) {
		return 0
	}
	i++
	goto again
}

func PassJumpLoopNoExitLabel(xs []int) int {
	var picked int
	i := 0
again:
	if i >= len(xs) {
		return picked
	}
	picked = xs[i]
	i++
	goto again
}

func PassAppendAccumulation(xs []int) []int {
	var picked []int
	for _, x := range xs {
		picked = append(picked, x)
		break
	}
	return picked
}

func PassAppendAccumulationFor(xs []int) []int {
	var picked []int
	for i := range xs {
		picked = append(picked, xs[i])
		break
	}
	return picked
}
