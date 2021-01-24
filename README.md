# Build

Yet another implementation of Makefile concept in go.
Advantages over all the other available packages:
- go only, compile and commit to your repo
- no magic command discovery based on go source code, you explicitly
  declare paths and functions for your commands
- bash autocompletion supported
- command functions are executed using IoC container so they may receive
  any interfaces required to do the job
  
## Example

Take a look at [example/main.go](./blob/main/example/main.go)
  
## First compilation

This build system is written in pure go so you have to compile it 
using `go build` before first usage. Then you may commit this into your repo
like you would normally do with your Makefile.

Taking this into consideration one of the commands available in this executable
should be responsible for compiling `build` itself so you don't need to use `go build`
later on when `build` is modified.

## Autocompletion

`build` supports autocompletion natively. To use it add this line to
your `~/.bashrc`:

```
complete -o nospace -C <path-to-your-build-binary> <alias-used-to-run-build>
```

Assuming your build executable file is named `build` and is available in your `PATH`
the exact `complete` is this:

```
complete -o nospace -C build build
```

## Executing commands

Commands are organised in paths similar to the one in normal filesystem.
Some examples how commands may be structured:

```
build tools/apiClient
build deploy/db
build tests/backend/web-server
build lint
```

You may specify many commands at once:

```
build tests deploy
```

They are executed in specified order. This will save some time if both commands execute same dependencies.

### Dependencies

Every command may specify dependencies - other commands which have to finish before the actual one may continue.
It allows you to move some code common to many commands to another function.

If many commands require the same dependency, it is executed once. 

Dependencies are executed one by one in order.

If circular dependency is detected error is raised.

## Errors

`build` always panics on first failure.


