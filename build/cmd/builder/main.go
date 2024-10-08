package main

import (
	"github.com/outofforest/build/v2"
	"github.com/outofforest/build/v2/pkg/tools/git"
	"github.com/outofforest/tools"
)

func main() {
	build.RegisterCommands(
		build.Commands,
		git.Commands,
		commands,
	)
	tools.Main()
}
