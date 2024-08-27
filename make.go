package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/outofforest/build/v2/pkg/tools"
	"github.com/outofforest/build/v2/pkg/types"
	"github.com/outofforest/logger"
	"github.com/outofforest/run"
)

const maxStack = 100

var defaultCommandRegistry = newCommandRegistry()

// Main receives configuration and runs registeredCommands
func Main(name, version string) {
	commands := defaultCommandRegistry.commands
	run.New().Run("build", func(ctx context.Context) error {
		flags := logger.Flags(logger.DefaultConfig, "build")
		if err := flags.Parse(os.Args[1:]); err != nil {
			return err
		}

		if isAutocomplete() {
			autocompleteDo(commands)
			return nil
		}

		if len(flags.Args()) == 0 {
			listCommands(commands)
			return nil
		}

		ctx = tools.WithVersion(tools.WithName(ctx, name), version)
		changeWorkingDir()
		setPath(ctx)
		return execute(ctx, commands, flags.Args())
	})
}

// RegisterCommands registers registeredCommands.
func RegisterCommands(commands ...map[string]types.Command) {
	if err := defaultCommandRegistry.RegisterCommands(commands); err != nil {
		panic(err)
	}
}

func execute(ctx context.Context, commands map[string]types.Command, paths []string) error {
	pathsTrimmed := make([]string, 0, len(paths))
	for _, p := range paths {
		if p[len(p)-1] == '/' {
			p = p[:len(p)-1]
		}
		pathsTrimmed = append(pathsTrimmed, p)
	}

	executed := map[reflect.Value]bool{}
	stack := map[reflect.Value]bool{}

	errReturn := errors.New("return")
	errChan := make(chan error, 1)
	var depsFunc types.DepsFunc
	worker := func(queue <-chan types.CommandFunc, done chan<- struct{}) {
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
					err = cmd(ctx, depsFunc)
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
	depsFunc = func(deps ...types.CommandFunc) {
		queue := make(chan types.CommandFunc)
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

	initDeps := make([]types.CommandFunc, 0, len(pathsTrimmed))
	for _, p := range pathsTrimmed {
		cmd, exists := commands[p]
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

func isAutocomplete() bool {
	_, ok := autocompletePrefix()
	return ok
}

func listCommands(commands map[string]types.Command) {
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
	lo.Must0(os.Setenv("PATH", projectBinDir+":"+toolBinDir+":"+path))
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

func autocompleteDo(commands map[string]types.Command) {
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

func paths(commands map[string]types.Command) []string {
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
	lo.Must0(os.Chdir(filepath.Dir(filepath.Dir(filepath.Dir(lo.Must(filepath.EvalSymlinks(lo.Must(os.Executable()))))))))
}

func newCommandRegistry() commandRegistry {
	return commandRegistry{
		commands: map[string]types.Command{},
	}
}

type commandRegistry struct {
	commands map[string]types.Command
}

func (cr commandRegistry) RegisterCommands(commands []map[string]types.Command) error {
	for _, commandSet := range commands {
		for path := range commandSet {
			if _, exists := cr.commands[path]; exists {
				return errors.Errorf("command %s has already been registered", path)
			}
		}
		for path, command := range commandSet {
			cr.commands[path] = command
		}
	}
	return nil
}
