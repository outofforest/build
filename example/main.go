package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/wojciech-malota-wojcik/build"
	"github.com/wojciech-malota-wojcik/ioc"
)

func commandA(ctx context.Context, deps build.DepsFunc) error {
	deps(commandAA, commandAB)

	fmt.Println("A executed")
	return nil
}

func commandAA(ctx context.Context, deps build.DepsFunc) error {
	fmt.Println("AA executed")
	return nil
}

func commandAB(ctx context.Context) error {
	fmt.Println("AB executed")
	return nil
}

func commandB(ctx context.Context) error {
	fmt.Println("B returning error")
	return errors.New("test error")
}

var commands = map[string]interface{}{
	"aCmd":       commandA,
	"aCmd/aaCmd": commandAA,
	"aCmd/abCmd": commandAB,
	"bCmd":       commandB,
}

func main() {
	c := ioc.New()
	c.Singleton(context.Background)

	build.Do("ExampleApp", build.NewIoCExecutor(commands, c))
}
