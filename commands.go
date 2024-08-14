package build

import (
	"context"

	"github.com/pkg/errors"
)

type CommandFunc func(ctx context.Context, deps DepsFunc) error

type Command struct {
	Description string
	Fn          CommandFunc
}

// DepsFunc represents function for executing dependencies
type DepsFunc func(deps ...CommandFunc)

func newCommandRegistry() commandRegistry {
	return commandRegistry{
		commands: map[string]Command{},
	}
}

type commandRegistry struct {
	commands map[string]Command
}

func (cr commandRegistry) RegisterCommands(commands map[string]Command) error {
	for path := range commands {
		if _, exists := cr.commands[path]; exists {
			return errors.Errorf("command %s has already been registered", path)
		}
	}
	for path, command := range commands {
		cr.commands[path] = command
	}
	return nil
}

var defaultCommandRegistry = newCommandRegistry()

// RegisterCommands registers registeredCommands.
func RegisterCommands(commands map[string]Command) {
	if err := defaultCommandRegistry.RegisterCommands(commands); err != nil {
		panic(err)
	}
}
