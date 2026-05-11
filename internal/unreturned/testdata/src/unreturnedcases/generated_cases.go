// Code generated for unreturned analyzer tests. DO NOT EDIT.

package unreturnedcases

var packageTemp int

func FailRangeLoop(xs []int) int {
	var picked int
	// Unreturned: Fail
	for _, x := range xs {
		picked = x
	}
	return picked
}

func FailThreeClauseFor(xs []string) string {
	var picked string
	// Unreturned: Fail
	for i := 0; i < len(xs); i++ {
		if xs[i] != "" {
			picked = xs[i]
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
	picked = xs[i]
	i++
	goto again
done:
	return picked
}

func PassRangeNoReadAfter(xs []int) {
	var picked int
	for _, x := range xs {
		picked = x
		_ = picked
	}
}

func PassOverwrittenBeforeRead(xs []int) int {
	var picked int
	for _, x := range xs {
		picked = x
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
