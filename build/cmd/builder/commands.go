package main

import (
	"github.com/outofforest/build/v2/pkg/types"
	"github.com/outofforest/tools/pkg/tools/golang"
)

var commands = map[string]types.Command{
	"lint": {
		Description: "Lints code",
		Fn:          golang.Lint,
	},
	"test": {
		Description: "Runs unit tests",
		Fn:          golang.UnitTests,
	},
	"tidy": {
		Description: "Tidies up the code",
		Fn:          golang.Tidy,
	},
}
