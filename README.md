# unreturned

`unreturned` is a `go vet` analyzer for results computed by assigning an outer
variable inside a loop and reading it after the loop exits.

That shape is usually clearer as a small function or closure that returns the
value directly. The analyzer reports the loop statement or jump-loop label that
assigns the result.

## Examples

Bad:

```go
var found string
prefix = strings.TrimSpace(prefix)
for _, name := range names {
	name = strings.TrimSpace(name)
	if name != "" && strings.HasPrefix(name, prefix) {
		found = name
		break
	}
}
return cmp.Or(found, "missing")
```

Return semantics and guards make it clear when the loop is done and that it is
searching or filtering, not accumulating; tracking reassignments is often
unnecessary complexity and burden on the reader, and that complexity grows
quickly as more variables are tracked. `unreturned` exists to catch that pattern
before it gets worse, especially in code written or edited by coding agents.

Good:

```go
find := func() string {
	prefix := strings.TrimSpace(prefix)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" && strings.HasPrefix(name, prefix) {
			return name
		}
	}
	return ""
}
return cmp.Or(find(), "missing")
```

Accumulation with `append` is fine:

```go
var matches []string
for _, name := range names {
	if strings.HasPrefix(name, prefix) {
		matches = append(matches, name)
	}
}
return matches
```

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
