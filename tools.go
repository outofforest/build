package build

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/outofforest/logger"
	"github.com/ridge/must"
	"go.uber.org/zap"
)

// Tool represents the tool to be required by the build system
type Tool struct {
	// Name is the name of the tool
	Name string

	// Version is the version of the tool
	Version string

	// IsGlobal instructs us to install the tool in global bin folder
	IsGlobal bool

	// URL is the url to the archive containing the tool
	URL string

	// Hash is the hash of the downloaded file
	Hash string

	// Binaries is the list of relative paths to binaries to install in local bin folder
	Binaries map[string]string
}

// InstallTools installs tools
func InstallTools(ctx context.Context, tools ...map[string]Tool) error {
	for _, t1 := range tools {
		for _, t2 := range t1 {
			if err := EnsureTool(ctx, t2); err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureTool ensures that tool exists, if not it is installed
func EnsureTool(ctx context.Context, tool Tool) error {
	binDir := projectBinDir()
	if tool.IsGlobal {
		binDir = toolBinDir(ctx)
	}
	toolDir := toolDir(ctx, tool)
	for dstBin, srcBin := range tool.Binaries {
		srcPath, err := filepath.Abs(toolDir + "/" + srcBin)
		if err != nil {
			return install(ctx, tool)
		}

		dstPath, err := filepath.Abs(binDir + "/" + dstBin)
		if err != nil {
			return install(ctx, tool)
		}

		realPath, err := filepath.EvalSymlinks(dstPath)
		if err != nil || realPath != srcPath {
			return install(ctx, tool)
		}
	}
	return nil
}

func install(ctx context.Context, tool Tool) (retErr error) {
	toolDir := toolDir(ctx, tool)
	ctx = logger.With(ctx, zap.String("name", tool.Name), zap.String("version", tool.Version),
		zap.String("url", tool.URL), zap.String("path", toolDir))
	log := logger.Get(ctx)
	log.Info("Installing tool")

	resp, err := http.DefaultClient.Do(must.HTTPRequest(http.NewRequestWithContext(ctx, http.MethodGet, tool.URL, nil)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	hasher, expectedChecksum := hasher(tool.Hash)
	reader := io.TeeReader(resp.Body, hasher)
	if err := os.RemoveAll(toolDir); err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		panic(err)
	}
	defer func() {
		if retErr != nil {
			must.OK(os.RemoveAll(toolDir))
		}
	}()

	if err := store(tool.URL, reader, toolDir); err != nil {
		return err
	}

	actualChecksum := fmt.Sprintf("%02x", hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return errors.Errorf("checksum does not match for tool %s, expected: %s, actual: %s, url: %s", tool.Name,
			expectedChecksum, actualChecksum, tool.URL)
	}

	binDir := projectBinDir()
	if tool.IsGlobal {
		binDir = toolBinDir(ctx)
	}
	for dstBin, srcBin := range tool.Binaries {
		srcPath := toolDir + "/" + srcBin
		dstPath := binDir + "/" + dstBin
		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			panic(err)
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil && !os.IsExist(err) {
			panic(err)
		}

		must.OK(os.Symlink(srcPath, dstPath))
		must.Any(filepath.EvalSymlinks(dstPath))
	}

	log.Info("Tool installed")
	return nil
}

func hasher(hashStr string) (hash.Hash, string) {
	parts := strings.SplitN(hashStr, ":", 2)
	if len(parts) != 2 {
		panic(errors.Errorf("incorrect checksum format: %s", hashStr))
	}
	hashAlgorithm := parts[0]
	checksum := parts[1]

	var hasher hash.Hash
	switch hashAlgorithm {
	case "sha256":
		hasher = sha256.New()
	default:
		panic(errors.Errorf("unsupported hashing algorithm: %s", hashAlgorithm))
	}

	return hasher, strings.ToLower(checksum)
}

func store(url string, reader io.Reader, path string) error {
	switch {
	case strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz"):
		var err error
		reader, err = gzip.NewReader(reader)
		if err != nil {
			return err
		}
		return untar(reader, path)
	case strings.HasSuffix(url, ".zip"):
		return unzip(reader, path)
	default:
		f, err := os.OpenFile(filepath.Join(path, filepath.Base(url)), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
		if err != nil {
			return errors.WithStack(err)
		}
		defer f.Close()
		_, err = io.Copy(f, reader)
		return errors.WithStack(err)
	}
}

func untar(reader io.Reader, path string) error {
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}
		header.Name = path + "/" + header.Name

		// We take mode from header.FileInfo().Mode(), not from header.Mode because they may be in different formats (meaning of bits may be different).
		// header.FileInfo().Mode() returns compatible value.
		mode := header.FileInfo().Mode()

		switch {
		case header.Typeflag == tar.TypeDir:
			if err := os.MkdirAll(header.Name, mode); err != nil && !os.IsExist(err) {
				return err
			}
		case header.Typeflag == tar.TypeReg:
			if err := ensureDir(header.Name); err != nil {
				return err
			}
			f, err := os.OpenFile(header.Name, os.O_CREATE|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			_ = f.Close()
			if err != nil {
				return err
			}
		case header.Typeflag == tar.TypeSymlink:
			if err := ensureDir(header.Name); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, header.Name); err != nil {
				return err
			}
		case header.Typeflag == tar.TypeLink:
			header.Linkname = path + "/" + header.Linkname
			if err := ensureDir(header.Name); err != nil {
				return err
			}
			if err := ensureDir(header.Linkname); err != nil {
				return err
			}
			// linked file may not exist yet, so let's create it - i will be overwritten later
			f, err := os.OpenFile(header.Linkname, os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				if !os.IsExist(err) {
					return err
				}
			} else {
				_ = f.Close()
			}
			if err := os.Link(header.Linkname, header.Name); err != nil {
				return err
			}
		default:
			return errors.Errorf("unsupported file type: %d", header.Typeflag)
		}
	}
}

func unzip(reader io.Reader, path string) error {
	// To unzip archive it is required to store it entirely on disk or in memory.
	// Zip does not support unzipping from one-way reader.
	// Here we store entire file in memory, so it's feasible only for small archives.
	archive, err := io.ReadAll(reader)
	if err != nil {
		return errors.WithStack(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		panic(err)
	}

	for _, f := range zr.File {
		filePath := filepath.Join(path, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, f.Mode()); err != nil {
				return errors.WithStack(err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return errors.WithStack(err)
		}

		err := func() error {
			fileInArchive, err := f.Open()
			if err != nil {
				return errors.WithStack(err)
			}
			defer fileInArchive.Close()

			//nolint:nosnakecase // Imported constants
			dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return errors.WithStack(err)
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, fileInArchive)
			return errors.WithStack(err)
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

func envDir(ctx context.Context) string {
	return must.String(os.UserCacheDir()) + "/" + GetName(ctx)
}

func toolBinDir(ctx context.Context) string {
	dir := filepath.Join(envDir(ctx), "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	return must.String(filepath.Abs(dir))
}

func projectBinDir() string {
	if err := os.MkdirAll("./bin", 0o755); err != nil {
		panic(err)
	}
	return must.String(filepath.Abs("./bin"))
}

func toolDir(ctx context.Context, tool Tool) string {
	return envDir(ctx) + "/" + tool.Name + "-" + tool.Version
}

func ensureDir(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); !os.IsExist(err) {
		return err
	}
	return nil
}
