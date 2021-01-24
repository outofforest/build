package build

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/wojciech-malota-wojcik/ioc"
)

// CommandFunc represents executable command
type CommandFunc func(ctx context.Context) error

// DepsFunc represents function for executing dependencies
type DepsFunc func(deps ...interface{})

// Executor defines interface of command executor
type Executor interface {
	// Paths lists all available command paths
	Paths() []string

	// Execute executes commands by their paths
	Execute(paths []string)
}

// NewIoCExecutor returns new executor using IoC container to resolve parameters of commands
func NewIoCExecutor(commands map[string]interface{}, c *ioc.Container) Executor {
	return &iocExecutor{c: c, commands: commands}
}

type iocExecutor struct {
	c        *ioc.Container
	commands map[string]interface{}
}

func (e *iocExecutor) Paths() []string {
	paths := make([]string, 0, len(e.commands))
	for path := range e.commands {
		paths = append(paths, path)
	}
	return paths
}

func (e *iocExecutor) Execute(paths []string) {
	executed := map[reflect.Value]bool{}
	stack := make([]reflect.Value, 0, 10)

	c := e.c.SubContainer()
	c.Singleton(func() DepsFunc {
		return func(deps ...interface{}) {
			for _, cmd := range deps {
				e.execute(c, cmd, executed, &stack)
			}
		}
	})

	for _, p := range paths {
		if e.commands[p] == nil {
			panic(fmt.Sprintf("command %s does not exist", p))
		}
		e.execute(c, e.commands[p], executed, &stack)
	}
}

func (e *iocExecutor) execute(c *ioc.Container, cmd interface{}, executed map[reflect.Value]bool, stack *[]reflect.Value) {
	cmdValue := reflect.ValueOf(cmd)
	if executed[cmdValue] {
		return
	}
	for _, s := range *stack {
		if s == cmdValue {
			panic("dependency cycle")
		}
	}

	*stack = append(*stack, cmdValue)

	var err error
	c.Call(cmd, &err)
	if err != nil {
		panic(err)
	}

	*stack = (*stack)[:len(*stack)-1]
	executed[cmdValue] = true
}

const help = `Build environment for %[1]s
Put this to your .bashrc for autocompletion:
complete -o nospace -C %[2]s %[2]s
`

// Do receives configuration and runs commands
func Do(name string, resolver Executor) {
	if prefix, ok := autocompletePrefix(os.Args[0], os.Getenv("COMP_LINE"), os.Getenv("COMP_POINT")); ok {
		autocompleteDo(prefix, resolver.Paths(), os.Getenv("COMP_TYPE"))
		return
	}
	if len(os.Args) == 1 {
		if _, err := fmt.Fprintf(os.Stderr, help, name, os.Args[0]); err != nil {
			panic(err)
		}
		return
	}
	execute(os.Args[1:], resolver)
}

func execute(paths []string, resolver Executor) {
	pathsTrimmed := make([]string, 0, len(paths))
	for _, p := range paths {
		if p[len(p)-1:] == "/" {
			p = p[:len(p)-1]
		}
		pathsTrimmed = append(pathsTrimmed, p)
	}
	resolver.Execute(pathsTrimmed)
}

func autocompletePrefix(exeName string, cLine, cPoint string) (string, bool) {
	if cLine == "" || cPoint == "" {
		return "", false
	}

	cPointInt, err := strconv.ParseInt(cPoint, 10, 64)
	if err != nil {
		panic(err)
	}

	prefix := strings.TrimLeft(cLine[:cPointInt], exeName)
	lastSpace := strings.LastIndex(prefix, " ") + 1
	return prefix[lastSpace:], true
}

func autocompleteDo(prefix string, paths []string, cType string) {
	choices := choicesForPrefix(paths, prefix)
	switch cType {
	case "9":
		startPos := strings.LastIndex(prefix, "/") + 1
		prefix = prefix[:startPos]
		if len(choices) == 1 {
			for choice, children := range choices {
				if children {
					choice += "/"
				}
				fmt.Println(prefix + choice)
			}
		} else if chPrefix := longestPrefix(choices); chPrefix != "" {
			fmt.Println(prefix + chPrefix)
		}
	case "63":
		if len(choices) > 1 {
			for choice := range choices {
				fmt.Println(choice)
			}
		}
	}
}

func choicesForPrefix(paths []string, prefix string) map[string]bool {
	startPos := strings.LastIndex(prefix, "/") + 1
	choices := map[string]bool{}
	for _, path := range paths {
		if strings.HasPrefix(path, prefix) {
			choice := path[startPos:]
			endPos := strings.Index(choice, "/")
			children := false
			if endPos != -1 {
				choice = choice[:endPos]
				children = true
			}
			choices[choice] = children
		}
	}
	return choices
}

func longestPrefix(choices map[string]bool) string {
	if len(choices) == 0 {
		return ""
	}
	prefix := ""
	for i := 0; true; i++ {
		var ch uint8
		for choice := range choices {
			if i >= len(choice) {
				return prefix
			}
			if ch == 0 {
				ch = choice[i]
				continue
			}
			if choice[i] != ch {
				return prefix
			}
		}
		prefix += string(ch)
	}
	return prefix
}
