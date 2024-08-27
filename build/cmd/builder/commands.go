package main

import (
	"context"

	"github.com/outofforest/build/v2/pkg/types"
)

var commands = map[string]types.Command{
	"lint": {
		Description: "Lints code",
		Fn: func(_ context.Context, _ types.DepsFunc) error {
			return nil
		},
	},
	"test": {
		Description: "Runs unit tests",
		Fn: func(_ context.Context, _ types.DepsFunc) error {
			return nil
		},
	},
	"tidy": {
		Description: "Tidies up the code",
		Fn: func(_ context.Context, _ types.DepsFunc) error {
			return nil
		},
	},
	"git/isclean": {
		Description: "Starts localnet",
		Fn: func(_ context.Context, _ types.DepsFunc) error {
			return nil
		},
	},
}
