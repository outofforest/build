package main

import (
	"github.com/outofforest/build"
)

func main() {
	build.RegisterCommands(
		build.Commands,
		commands,
	)
	build.Main("outofforest-build")
}
