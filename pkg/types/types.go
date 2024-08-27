package types

import "context"

// CommandFunc represents function executing command.
type CommandFunc func(ctx context.Context, deps DepsFunc) error

// Command defines command.
type Command struct {
	Description string
	Fn          CommandFunc
}

// DepsFunc represents function for executing dependencies
type DepsFunc func(deps ...CommandFunc)
