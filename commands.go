package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/samber/lo"

	"github.com/outofforest/build/v2/pkg/tools"
	"github.com/outofforest/build/v2/pkg/types"
	"github.com/outofforest/libexec"
	"github.com/outofforest/tools/pkg/tools/golang"
)

// Commands is the list of standard commands useful for every environment.
var Commands = map[string]types.Command{
	"enter": {
		Description: "Enters the environment",
		Fn:          enter,
	},
	"build/me": {
		Description: "Rebuilds the builder",
		Fn: func(ctx context.Context, deps types.DepsFunc) error {
			return golang.Build(ctx, deps, golang.BuildConfig{
				Platform:      tools.PlatformLocal,
				PackagePath:   "build/cmd/builder",
				BinOutputPath: filepath.Join("bin", ".cache", filepath.Base(lo.Must(os.Executable()))),
			})
		},
	},
	"tools/setup": {
		Description: "Installs all the tools for the host operating system",
		Fn:          tools.EnsureAll,
	},
	"tools/verify": {
		Description: "Verifies the checksums of all the tools",
		Fn:          tools.VerifyChecksums,
	},
}

func enter(ctx context.Context, deps types.DepsFunc) error {
	bash := exec.Command("bash")
	bash.Env = append(os.Environ(),
		"PS1=("+tools.GetName(ctx)+`) [\u@\h \W]\$ `,
		fmt.Sprintf("PATH=%s:%s:%s",
			filepath.Join(lo.Must(filepath.EvalSymlinks(lo.Must(filepath.Abs(".")))), "bin"),
			filepath.Join(tools.VersionDir(ctx, tools.PlatformLocal), "bin"),
			os.Getenv("PATH")),
	)
	bash.Stdin = os.Stdin
	bash.Stdout = os.Stdout
	bash.Stderr = os.Stderr
	err := libexec.Exec(ctx, bash)
	if bash.ProcessState != nil && bash.ProcessState.ExitCode() != 0 {
		return nil
	}
	return err
}
