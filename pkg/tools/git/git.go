package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/mod/semver"

	"github.com/outofforest/build/v2/pkg/types"
	"github.com/outofforest/libexec"
)

// IsStatusClean checks that there are no uncommitted files in the repo.
func IsStatusClean(ctx context.Context, _ types.DepsFunc) error {
	clean, info, err := status(ctx)
	if err != nil {
		return err
	}
	if !clean {
		fmt.Println(info)
		return errors.New("repository contains uncommitted changes")
	}
	return nil
}

// HeadHash returns hash of the latest commit in the repository.
func HeadHash(ctx context.Context, repoPath string) (string, error) {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	cmd.Stdout = buf
	if err := libexec.Exec(ctx, cmd); err != nil {
		return "", errors.Wrap(err, "git command failed")
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// DirtyHeadHash returns hash of the latest commit in the repository, adding "-dirty" suffix
// if there are uncommitted changes.
func DirtyHeadHash(ctx context.Context, repoPath string) (string, error) {
	hash, err := HeadHash(ctx, repoPath)
	if err != nil {
		return "", err
	}

	clean, _, err := status(ctx)
	if err != nil {
		return "", err
	}
	if !clean {
		hash += "-dirty"
	}

	return hash, nil
}

// HeadTags returns the list of tags applied to the latest commit.
func HeadTags(ctx context.Context, repoPath string) ([]string, error) {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "tag", "--points-at", "HEAD")
	cmd.Dir = repoPath
	cmd.Stdout = buf
	if err := libexec.Exec(ctx, cmd); err != nil {
		return nil, errors.Wrap(err, "git command failed")
	}
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n"), nil
}

// VersionFromTag returns version taken from tag present in the commit.
func VersionFromTag(ctx context.Context, repoPath string) (string, error) {
	tags, err := HeadTags(ctx, repoPath)
	if err != nil {
		return "", err
	}

	for _, tag := range tags {
		if semver.IsValid(tag) {
			return tag, nil
		}
	}
	return "", nil
}

func status(ctx context.Context) (bool, string, error) {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "status", "-s")
	cmd.Stdout = buf
	if err := libexec.Exec(ctx, cmd); err != nil {
		return false, "", errors.Wrap(err, "git command failed")
	}
	if buf.Len() > 0 {
		return false, buf.String(), nil
	}
	return true, "", nil
}
