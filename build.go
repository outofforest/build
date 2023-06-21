package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/libexec"
	"github.com/outofforest/logger"
	"github.com/outofforest/run"
	"github.com/ridge/must"
)

const maxStack = 100

type Command struct {
	Description string
	Fn          interface{}
}

// DepsFunc represents function for executing dependencies
type DepsFunc func(deps ...interface{})

// Executor defines interface of command executor
type Executor interface {
	// Execute executes commands by their paths
	Execute(ctx context.Context, name string, paths []string) error
}

// NewIoCExecutor returns new executor using IoC container to resolve parameters of commands
func NewIoCExecutor(commands map[string]Command, c *ioc.Container) Executor {
	return &iocExecutor{c: c, commands: commands}
}

type iocExecutor struct {
	c        *ioc.Container
	commands map[string]Command
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
					err = errors.Errorf("command panicked: %v", r)
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
	loop:
		for _, d := range deps {
			select {
			case <-done:
				break loop
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
		cmd, exists := e.commands[p]
		if !exists {
			return errors.Errorf("build: command %s does not exist", p)
		}
		initDeps = append(initDeps, cmd.Fn)
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

// Main receives configuration and runs commands
func Main(name string, containerBuilder func(c *ioc.Container), commands map[string]Command) {
	run.Tool("build", containerBuilder, func(ctx context.Context, c *ioc.Container) error {
		flags := logger.Flags(logger.ToolDefaultConfig, "build")
		if err := flags.Parse(os.Args[1:]); err != nil {
			return err
		}

		if len(os.Args) >= 2 && os.Args[1] == "@" {
			listCommands(commands)
			return nil
		}

		executor := NewIoCExecutor(commands, c)
		if isAutocomplete() {
			autocompleteDo(commands)
			return nil
		}

		ctx = withName(ctx, name)
		changeWorkingDir()
		setPath(ctx)
		if len(flags.Args()) == 0 {
			return activate(ctx, name)
		}
		return execute(ctx, name, flags.Args(), executor)
	})
}

func isAutocomplete() bool {
	_, ok := autocompletePrefix()
	return ok
}

func listCommands(commands map[string]Command) {
	paths := paths(commands)
	var maxLen int
	for _, path := range paths {
		if len(path) > maxLen {
			maxLen = len(path)
		}
	}
	fmt.Println("\n Available commands:")
	fmt.Println()
	for _, path := range paths {
		fmt.Printf(fmt.Sprintf(`   %%-%ds`, maxLen)+"  %s\n", path, commands[path].Description)
	}
	fmt.Println("")
}

func setPath(ctx context.Context) {
	toolBinDir := toolBinDir(ctx)
	projectBinDir := projectBinDir()
	var path string
	for _, p := range strings.Split(os.Getenv("PATH"), ":") {
		if !strings.HasPrefix(p, toolBinDir) && !strings.HasPrefix(p, projectBinDir) {
			if path != "" {
				path += ":"
			}
			path += p
		}
	}
	must.OK(os.Setenv("PATH", projectBinDir+":"+toolBinDir+":"+path))
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

func autocompletePrefix() (string, bool) {
	exeName := os.Args[0]
	cLine := os.Getenv("COMP_LINE")
	cPoint := os.Getenv("COMP_POINT")

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

func autocompleteDo(commands map[string]Command) {
	prefix, _ := autocompletePrefix()
	choices := choicesForPrefix(paths(commands), prefix)
	switch os.Getenv("COMP_TYPE") {
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

func paths(commands map[string]Command) []string {
	paths := make([]string, 0, len(commands))
	for path := range commands {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
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
	must.OK(os.Chdir(filepath.Dir(filepath.Dir(filepath.Dir(must.String(filepath.EvalSymlinks(must.String(os.Executable()))))))))
}
