package build

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outofforest/build/pkg/types"
)

var r = map[int]string{}

func cmdA(_ context.Context, deps types.DepsFunc) error {
	deps(cmdAA, cmdAB)
	r[len(r)] = "a"
	return nil
}

func cmdAA(_ context.Context, deps types.DepsFunc) error {
	deps(cmdAC)
	r[len(r)] = "aa"
	return nil
}

func cmdAB(_ context.Context, deps types.DepsFunc) error {
	deps(cmdAC)
	r[len(r)] = "ab"
	return nil
}

func cmdAC(_ context.Context, deps types.DepsFunc) error {
	r[len(r)] = "ac"
	return nil
}

func cmdB(_ context.Context, deps types.DepsFunc) error {
	return errors.New("error")
}

func cmdC(_ context.Context, deps types.DepsFunc) error {
	deps(cmdD)
	return nil
}

func cmdD(_ context.Context, deps types.DepsFunc) error {
	deps(cmdC)
	return nil
}

func cmdE(_ context.Context, deps types.DepsFunc) error {
	panic("panic")
}

func cmdF(ctx context.Context, deps types.DepsFunc) error {
	<-ctx.Done()
	return ctx.Err()
}

var tCtx = context.Background()

func setup(ctx context.Context) (func(paths []string) error, map[int]string) {
	r = map[int]string{}
	return func(paths []string) error {
		return execute(ctx, map[string]types.Command{
			"a":    {Fn: cmdA},
			"a/aa": {Fn: cmdAA},
			"a/ab": {Fn: cmdAB},
			"b":    {Fn: cmdB},
			"c":    {Fn: cmdC},
			"d":    {Fn: cmdD},
			"e":    {Fn: cmdE},
			"f":    {Fn: cmdF},
		}, paths)
	}, r
}

func TestRootCommand(t *testing.T) {
	exe, r := setup(tCtx)
	require.NoError(t, exe([]string{"a"}))

	assert.Len(t, r, 4)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
	assert.Equal(t, "ab", r[2])
	assert.Equal(t, "a", r[3])
}

func TestChildCommand(t *testing.T) {
	exe, r := setup(tCtx)
	require.NoError(t, exe([]string{"a/aa"}))

	assert.Len(t, r, 2)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
}

func TestTwoCommands(t *testing.T) {
	exe, r := setup(tCtx)
	require.NoError(t, exe([]string{"a/aa", "a/ab"}))

	assert.Len(t, r, 3)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
	assert.Equal(t, "ab", r[2])
}

func TestCommandWithSlash(t *testing.T) {
	exe, r := setup(tCtx)
	require.NoError(t, exe([]string{"a/aa/"}))

	assert.Len(t, r, 2)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
}

func TestCommandsAreExecutedOnce(t *testing.T) {
	exe, r := setup(tCtx)
	require.NoError(t, exe([]string{"a", "a"}))

	assert.Len(t, r, 4)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
	assert.Equal(t, "ab", r[2])
	assert.Equal(t, "a", r[3])
}

func TestCommandReturnsError(t *testing.T) {
	exe, _ := setup(tCtx)
	require.Error(t, exe([]string{"b"}))
}

func TestCommandPanics(t *testing.T) {
	exe, _ := setup(tCtx)
	require.Error(t, exe([]string{"e"}))
}

func TestErrorOnCyclicDependencies(t *testing.T) {
	exe, _ := setup(tCtx)
	require.Error(t, exe([]string{"c"}))
}

func TestRootCommandDoesNotExist(t *testing.T) {
	exe, _ := setup(tCtx)
	require.Error(t, exe([]string{"z"}))
}

func TestChildCommandDoesNotExist(t *testing.T) {
	exe, _ := setup(tCtx)
	require.Error(t, exe([]string{"a/z"}))
}

func TestCommandStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(tCtx)
	cancel()
	exe, _ := setup(ctx)
	err := exe([]string{"f"})
	assert.Equal(t, context.Canceled, err)
}
