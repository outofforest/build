package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/libexec"
	"github.com/ridge/must"
)

const maxStack = 100

// CommandFunc represents executable command
type CommandFunc func(ctx context.Context) error

// DepsFunc represents function for executing dependencies
type DepsFunc func(deps ...interface{})

// Executor defines interface of command executor
type Executor interface {
	// Paths lists all available command paths
	Paths() []string

	// Execute executes commands by their paths
	Execute(ctx context.Context, name string, paths []string) error
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

func (e *iocExecutor) Execute(ctx context.Context, name string, paths []string) error {
	executed := map[reflect.Value]bool{}
	stack := map[reflect.Value]bool{}
	c := e.c.SubContainer()
	c.Singleton(func() context.Context {
		return withName(ctx, name)
	})

	errReturn := errors.New("return")
	errChan := make(chan error, 1)
	worker := func(queue <-chan interface{}, done chan<- struct{}) {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				var err error
				if err2, ok := r.(error); ok {
					if err2 == errReturn {
						return
					}
					err = err2
				} else {
					err = fmt.Errorf("command panicked: %v", r)
				}
				errChan <- err
				close(errChan)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				close(errChan)
				return
			case cmd, ok := <-queue:
				if !ok {
					return
				}
				cmdValue := reflect.ValueOf(cmd)
				if executed[cmdValue] {
					continue
				}
				var err error
				switch {
				case stack[cmdValue]:
					err = errors.New("build: dependency cycle detected")
				case len(stack) >= maxStack:
					err = errors.New("build: maximum length of stack reached")
				default:
					stack[cmdValue] = true
					c.Call(cmd, &err)
					delete(stack, cmdValue)
					executed[cmdValue] = true
				}
				if err != nil {
					errChan <- err
					close(errChan)
					return
				}
			}
		}
	}
	depsFunc := func(deps ...interface{}) {
		queue := make(chan interface{})
		done := make(chan struct{})
		go worker(queue, done)
		for _, d := range deps {
			select {
			case <-done:
				break
			case queue <- d:
			}
		}
		close(queue)
		<-done
		if len(errChan) > 0 {
			panic(errReturn)
		}
	}
	c.Singleton(func() DepsFunc {
		return depsFunc
	})

	initDeps := make([]interface{}, 0, len(paths))
	for _, p := range paths {
		if e.commands[p] == nil {
			return fmt.Errorf("build: command %s does not exist", p)
		}
		initDeps = append(initDeps, e.commands[p])
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok && err == errReturn {
					return
				}
				panic(r)
			}
		}()
		depsFunc(initDeps...)
	}()
	if len(errChan) > 0 {
		return <-errChan
	}
	return nil
}

// Do receives configuration and runs commands
func Do(ctx context.Context, name string, executor Executor) error {
	if autocomplete(executor) {
		return nil
	}

	ctx = withName(ctx, name)
	changeWorkingDir()
	setPath()
	if len(os.Args) == 1 {
		return activate(ctx, name)
	}
	return execute(ctx, name, os.Args[1:], executor)
}

// Autocomplete serves bash autocomplete functionality.
// Returns true if autocomplete was requested and false otherwise.
func autocomplete(executor Executor) bool {
	if prefix, ok := autocompletePrefix(os.Args[0], os.Getenv("COMP_LINE"), os.Getenv("COMP_POINT")); ok {
		autocompleteDo(prefix, executor.Paths(), os.Getenv("COMP_TYPE"))
		return true
	}
	return false
}

func setPath() {
	binDir := binDir()
	var path string
	for _, p := range strings.Split(os.Getenv("PATH"), ":") {
		if !strings.HasPrefix(p, binDir) {
			if path != "" {
				path += ":"
			}
			path += p
		}
	}
	must.OK(os.Setenv("PATH", binToolsDir()+":"+binDir+":"+path))
}

func activate(ctx context.Context, name string) error {
	bash := exec.Command("bash")
	bash.Env = append(os.Environ(),
		fmt.Sprintf("PS1=%s", "("+name+`) [\u@\h \W]\$ `),
	)
	bash.Stdin = os.Stdin
	bash.Stdout = os.Stdout
	bash.Stderr = os.Stderr
	err := libexec.Exec(ctx, bash)
	if bash.ProcessState != nil && bash.ProcessState.ExitCode() != 0 {
		return nil
	}
	return err
}

func execute(ctx context.Context, name string, paths []string, executor Executor) error {
	pathsTrimmed := make([]string, 0, len(paths))
	for _, p := range paths {
		if p[len(p)-1] == '/' {
			p = p[:len(p)-1]
		}
		pathsTrimmed = append(pathsTrimmed, p)
	}
	return executor.Execute(ctx, name, pathsTrimmed)
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
				} else {
					choice += " "
				}
				fmt.Println(prefix + choice)
			}
		} else if chPrefix := longestPrefix(choices); chPrefix != "" {
			fmt.Println(prefix + chPrefix)
		}
	case "63":
		if len(choices) > 1 {
			for choice, children := range choices {
				if children {
					choice += "/"
				}
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
			if _, ok := choices[choice]; !ok || children {
				choices[choice] = children
			}
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

func changeWorkingDir() {
	must.OK(os.Chdir(filepath.Dir(filepath.Dir(must.String(filepath.EvalSymlinks(must.String(os.Executable())))))))
}
