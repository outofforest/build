# Build

Yet another implementation of Makefile concept in go.
Advantages over all the other available packages:
- go only, except small bash script
- no magic command discovery based on go source code, you explicitly
  declare paths and functions for your commands
- bash autocompletion supported
- set of standard commands
  
## Example

Take a look at [example/main.go](../main/example/main.go)
  
## Compilation

This build system is written in pure go so you have to compile it 
using `go build` before first usage. Take a look at the [script](./bin/builder) I use in my setup.

I configure this script using `alias` feature delivered by `bash` in my `~/.bashrc`:

```
alias projname="<path-to-project>/bin/builder"
```

Then you may execute:

```
$ projname <command> <command>
```

to execute commands.

## Autocompletion

`build` supports autocompletion natively. To use it add this line to
your `~/.bashrc`:

```
complete -o nospace -C projname projname
```

assuming you defined `projname` alias specified above.

## Executing commands

Commands are organised in paths similar to the one in normal filesystem.
Some examples how commands may be structured:

```
projname tools/apiClient
projname deploy/db
projname tests/backend/web-server
projname lint
```

You may specify many commands at once:

```
projname tests deploy
```

They are executed in specified order. This will save some time if both commands execute same dependencies.

### Dependencies

Every command may specify dependencies - other commands which have to finish before the actual one may continue.
It allows you to move some code common to many commands to another function.

If many commands require the same dependency, it is executed once. 

Dependencies are executed one by one in order.

If circular dependency is detected error is raised.

## Other features

### List of commands

Execute

```
$ projname
```

to print available commands with their descriptions.

### Verbose logging

If you want to see more logs during command execution, use `-v` or `--verbose`:

```
$ projname <command> -v
```

## Errors

`build` always breaks on first failure.


