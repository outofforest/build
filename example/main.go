package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/outofforest/ioc/v2"

	"github.com/outofforest/build"
)

// Try running:
// go run . aCmd
// go run . aCmd/aaCmd
// go run . aCmd/abCmd
// go run . bCmd
// go run . cCmd

var commands = map[string]interface{}{
	"aCmd":       commandA,
	"aCmd/aaCmd": commandAA,
	"aCmd/abCmd": commandAB,
	"bCmd":       commandB,
	"cCmd":       commandC,
}

func main() {
	c := ioc.New()
	executor := build.NewIoCExecutor(commands, c)
	ctx := context.Background()
	c.Singleton(func() context.Context {
		return ctx
	})
	if err := build.Main(ctx, "env-name", executor); err != nil {
		fmt.Printf("Error: %s\n", err)
	}
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
