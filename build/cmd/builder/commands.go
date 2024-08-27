package main

import (
	"context"

	"github.com/outofforest/build"
)

var commands = map[string]build.Command{
	"lint": {
		Description: "Lints code",
		Fn: func(_ context.Context, _ build.DepsFunc) error {
			return nil
		},
	},
	"test": {
		Description: "Runs unit tests",
		Fn: func(_ context.Context, _ build.DepsFunc) error {
			return nil
		},
	},
	"tidy": {
		Description: "Tidies up the code",
		Fn: func(_ context.Context, _ build.DepsFunc) error {
			return nil
		},
	},
	"git/isclean": {
		Description: "Starts localnet",
		Fn: func(_ context.Context, _ build.DepsFunc) error {
			return nil
		},
	},
}
