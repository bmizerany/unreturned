# unreturned

`unreturned` is a `go vet` analyzer for results computed by assigning an outer
variable inside a loop and reading it after the loop exits.

That shape is usually clearer as a small function or closure that returns the
value directly. The analyzer reports the loop statement or jump-loop label that
assigns the result.

## Add to a Module

The module path is `blake.io/unreturned`. Add `unreturned` as a tool dependency:

```sh
go get -tool blake.io/unreturned/cmd/unreturned@latest
```

## Usage

Normal use is:

```sh
go tool unreturned ./...
```

To install the binary directly:

```sh
go install blake.io/unreturned/cmd/unreturned@latest
```

It can also be used as a `go vet` vettool:

```sh
go vet -vettool="$(go tool -n unreturned)" ./...
```

## License

MIT
