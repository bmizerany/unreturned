# unreturned

A loop that produces a value is a function in disguise.

`unreturned` is a `go vet` analyzer that catches that shape: a loop that
assigns to an outer variable, exits via `break` or `goto`, and is read after.
The fix is the same every time — extract the loop as a function and return the
value directly. The analyzer reports the loop statement or jump-loop label that
produces the result.

## Examples

Parsing a byte size like `"4mb"` into a number and a unit scale.

Bad:

```go
var units = []struct {
	suffix string
	scale  float64
}{
	{"gb", 1e9},
	{"mb", 1e6},
	{"kb", 1e3},
	{"b", 1},
}

func parseBytes(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	scale := float64(1)
	for _, u := range units {
		if rest, ok := strings.CutSuffix(s, u.suffix); ok {
			s = strings.TrimSpace(rest)
			scale = u.scale
			break
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return uint64(v * scale), nil
}
```

The loop smuggles two values out: `s`, possibly trimmed of its suffix, and
`scale`, possibly overridden. The default `scale := 1` lives several lines from
the assignment that conditionally replaces it; the mutation of `s` is hidden in
the middle of the loop body. To understand the `ParseFloat(s, 64)` that follows,
a reader has to hold both branches of the loop in their head. Add a third
outer variable and the burden compounds — every new piece of state is one more
thread to track across the loop boundary.

Good:

```go
func parseBytes(s string) (uint64, error) {
	s, scale := splitUnit(strings.TrimSpace(s))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return uint64(v * scale), nil
}

func splitUnit(s string) (string, float64) {
	for _, u := range units {
		if rest, ok := strings.CutSuffix(s, u.suffix); ok {
			return strings.TrimSpace(rest), u.scale
		}
	}
	return s, 1
}
```

`splitUnit` has one job, named. The default ("no suffix matched, scale 1")
sits next to the search that might replace it, instead of leaking out as a
zero-value default ten lines away. `parseBytes` reads top-to-bottom with no
state carried across a loop boundary.

Accumulation with `append` is fine — the loop is building, not producing a
single result:

```go
var matches []string
for _, name := range names {
	if strings.HasPrefix(name, prefix) {
		matches = append(matches, name)
	}
}
return matches
```

`unreturned` exists to catch the producing-loop pattern before it spreads,
especially in code written or edited by coding agents.

## Add to a Module

The module path is `blake.io/unreturned`. Add `unreturned` as a tool dependency:

```sh
go get -tool blake.io/unreturned@latest
```

## Usage

Normal use is:

```sh
go tool unreturned ./...
```

To install the binary directly:

```sh
go install blake.io/unreturned@latest
```

It can also be used as a `go vet` vettool:

```sh
go vet -vettool="$(go tool -n unreturned)" ./...
```

## License

MIT
