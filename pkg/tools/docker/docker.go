package docker

import (
	"context"
	"os/exec"

	"github.com/pkg/errors"

	"github.com/outofforest/build/pkg/helpers"
	"github.com/outofforest/build/pkg/types"
)

// Label used to tag docker resources created by localnet.
const (
	LabelKey   = "com.github.outofforest.build"
	LabelValue = "build"
)

// EnsureDocker verifies that docker is installed.
func EnsureDocker(_ context.Context, _ types.DepsFunc) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.Wrap(err, "docker command is not available in PATH")
	}
	return nil
}

// Cmd returns docker command.
func Cmd(args ...string) *exec.Cmd {
	return helpers.ToolCmd("docker", args)
}
