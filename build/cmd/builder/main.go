package main

import "github.com/outofforest/build/v2"

func main() {
	build.RegisterCommands(
		build.Commands,
		commands,
	)
	build.Main("outofforest-build", "devel")
}
