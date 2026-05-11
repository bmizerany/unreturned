// Code generated for unreturned analyzer tests. DO NOT EDIT.

package unreturnedcases

var packageTemp int

func rememberInt(int) {}

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
	}
	return packageTemp
}

func PassAppendAccumulation(xs []int) []int {
	var picked []int
	for _, x := range xs {
		picked = append(picked, x)
	}
	return picked
}

func PassAppendAccumulationFor(xs []int) []int {
	var picked []int
	for i := range xs {
		picked = append(picked, xs[i])
	}
	return picked
}
