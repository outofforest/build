package main

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/outofforest/build"
)

// Try running:
// go run . @
// go run . aCmd
// go run . aCmd/aaCmd
// go run . aCmd/abCmd
// go run . bCmd
// go run . cCmd

var commands = map[string]build.Command{
	"aCmd":       {Fn: commandA, Description: "This is commandA"},
	"aCmd/aaCmd": {Fn: commandAA, Description: "This is commandAA"},
	"aCmd/abCmd": {Fn: commandAB, Description: "This is commandAB"},
	"bCmd":       {Fn: commandB, Description: "This is commandB"},
	"cCmd":       {Fn: commandC, Description: "This is commandC"},
}

func main() {
	build.Main("env-name", nil, commands)
}

func commandA(ctx context.Context, deps build.DepsFunc) error {
	deps(commandAA, commandAB)

	fmt.Println("A executed")
	return nil
}

func commandAA(ctx context.Context) error {
	fmt.Println("AA executed")
	return nil
}

func commandAB(ctx context.Context) error {
	fmt.Println("AB executed")
	return nil
}

func commandB(ctx context.Context, deps build.DepsFunc) error {
	deps(commandBB)
	fmt.Println("B executed")
	return nil
}

func commandBB(ctx context.Context) error {
	fmt.Println("BB returning error")
	return errors.New("test error")
}

func commandC(ctx context.Context) error {
	panic("test panic")
}
