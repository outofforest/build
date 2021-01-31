package build

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wojciech-malota-wojcik/ioc"
)

type report map[int]string

func cmdA(r report, deps DepsFunc) error {
	deps(cmdAA, cmdAB)
	r[len(r)] = "a"
	return nil
}

func cmdAA(r report, deps DepsFunc) error {
	deps(cmdAC)
	r[len(r)] = "aa"
	return nil
}

func cmdAB(r report, deps DepsFunc) error {
	deps(cmdAC)
	r[len(r)] = "ab"
	return nil
}

func cmdAC(r report) error {
	r[len(r)] = "ac"
	return nil
}

func cmdB() error {
	return errors.New("error")
}

func cmdC(deps DepsFunc) error {
	deps(cmdD)
	return nil
}

func cmdD(deps DepsFunc) error {
	deps(cmdC)
	return nil
}

func cmdE() error {
	panic("panic")
}

func cmdF(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

var commands = map[string]interface{}{
	"a":    cmdA,
	"a/aa": cmdAA,
	"a/ab": cmdAB,
	"b":    cmdB,
	"c":    cmdC,
	"d":    cmdD,
	"e":    cmdE,
	"f":    cmdF,
}

var tCtx = context.Background()

func setup(ctx context.Context) (Executor, report) {
	r := report{}
	c := ioc.New()
	c.Singleton(func() report {
		return r
	})
	c.Singleton(func() context.Context {
		return ctx
	})
	return NewIoCExecutor(commands, c), r
}

func TestRootCommand(t *testing.T) {
	exe, r := setup(tCtx)
	require.NoError(t, execute(tCtx, []string{"a"}, exe))

	assert.Len(t, r, 4)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
	assert.Equal(t, "ab", r[2])
	assert.Equal(t, "a", r[3])
}

func TestChildCommand(t *testing.T) {
	exe, r := setup(tCtx)
	require.NoError(t, execute(tCtx, []string{"a/aa"}, exe))

	assert.Len(t, r, 2)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
}

func TestTwoCommands(t *testing.T) {
	exe, r := setup(tCtx)
	require.NoError(t, execute(tCtx, []string{"a/aa", "a/ab"}, exe))

	assert.Len(t, r, 3)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
	assert.Equal(t, "ab", r[2])
}

func TestCommandWithSlash(t *testing.T) {
	exe, r := setup(tCtx)
	require.NoError(t, execute(tCtx, []string{"a/aa/"}, exe))

	assert.Len(t, r, 2)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
}

func TestCommandsAreExecutedOnce(t *testing.T) {
	exe, r := setup(tCtx)
	require.NoError(t, execute(tCtx, []string{"a", "a"}, exe))

	assert.Len(t, r, 4)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
	assert.Equal(t, "ab", r[2])
	assert.Equal(t, "a", r[3])
}

func TestCommandReturnsError(t *testing.T) {
	exe, _ := setup(tCtx)
	require.Error(t, execute(tCtx, []string{"b"}, exe))
}

func TestCommandPanics(t *testing.T) {
	exe, _ := setup(tCtx)
	require.Error(t, execute(tCtx, []string{"e"}, exe))
}

func TestErrorOnCyclicDependencies(t *testing.T) {
	exe, _ := setup(tCtx)
	require.Error(t, execute(tCtx, []string{"c"}, exe))
}

func TestRootCommandDoesNotExist(t *testing.T) {
	exe, _ := setup(tCtx)
	require.Error(t, execute(tCtx, []string{"z"}, exe))
}

func TestChildCommandDoesNotExist(t *testing.T) {
	exe, _ := setup(tCtx)
	require.Error(t, execute(tCtx, []string{"a/z"}, exe))
}

func TestCommandStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(tCtx)
	cancel()
	exe, _ := setup(ctx)
	err := execute(ctx, []string{"f"}, exe)
	assert.Equal(t, context.Canceled, err)
}
