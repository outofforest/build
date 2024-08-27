package main

import (
	"github.com/outofforest/build"
)

func main() {
	build.RegisterCommands(
		commands,
	)
	build.Main("outofforest-build")
}
