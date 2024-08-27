package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"

	"github.com/outofforest/build/v2/pkg/types"
	"github.com/outofforest/libexec"
)

// IsStatusClean checks that there are no uncommitted files in the repo.
func IsStatusClean(ctx context.Context, _ types.DepsFunc) error {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "status", "-s")
	cmd.Stdout = buf
	if err := libexec.Exec(ctx, cmd); err != nil {
		return errors.Wrap(err, "git command failed")
	}
	if buf.Len() > 0 {
		fmt.Println(buf)
		return errors.New("repository contains uncommitted changes")
	}
	return nil
}
