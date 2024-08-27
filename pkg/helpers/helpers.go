package helpers

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

// CopyFile copies the file.
func CopyFile(dst, src string, perm os.FileMode) error {
	fr, err := os.Open(src)
	if err != nil {
		return errors.WithStack(err)
	}
	defer fr.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return errors.WithStack(err)
	}

	fw, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return errors.WithStack(err)
	}
	defer fw.Close()

	if _, err = io.Copy(fw, fr); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// OnModule iterates over all the modules in the source code.
func OnModule(fileName string, fn func(path string) error) error {
	return filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != fileName {
			return nil
		}
		return fn(filepath.Dir(path))
	})
}

// ToolCmd returns command executing a tool available in PATH.
func ToolCmd(tool string, args []string) *exec.Cmd {
	verifyTool(tool)
	return exec.Command(tool, args...)
}

func verifyTool(tool string) {
	if _, err := exec.LookPath(tool); err != nil {
		panic(errors.Errorf("%s is not available, please install it", tool))
	}
}
