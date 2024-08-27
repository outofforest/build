package main

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/outofforest/build/v2"
	"github.com/outofforest/build/v2/pkg/types"
)

// Try running:
// go run . --help
// go run . aCmd
// go run . aCmd/aaCmd
// go run . aCmd/abCmd
// go run . bCmd
// go run . cCmd

func main() {
	build.RegisterCommands(map[string]types.Command{
		"aCmd":       {Fn: commandA, Description: "This is commandA"},
		"aCmd/aaCmd": {Fn: commandAA, Description: "This is commandAA"},
		"aCmd/abCmd": {Fn: commandAB, Description: "This is commandAB"},
		"bCmd":       {Fn: commandB, Description: "This is commandB"},
		"cCmd":       {Fn: commandC, Description: "This is commandC"},
	})
	build.Main("env-name")
}

func commandA(ctx context.Context, deps types.DepsFunc) error {
	deps(commandAA, commandAB)

	fmt.Println("A executed")
	return nil
}

func commandAA(ctx context.Context, deps types.DepsFunc) error {
	fmt.Println("AA executed")
	return nil
}

func commandAB(ctx context.Context, deps types.DepsFunc) error {
	fmt.Println("AB executed")
	return nil
}

func commandB(ctx context.Context, deps types.DepsFunc) error {
	deps(commandBB)
	fmt.Println("B executed")
	return nil
}

func commandBB(ctx context.Context, deps types.DepsFunc) error {
	fmt.Println("BB returning error")
	return errors.New("test error")
}

func commandC(ctx context.Context, deps types.DepsFunc) error {
	panic("test panic")
}
