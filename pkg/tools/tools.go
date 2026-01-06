package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	goerrors "errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/outofforest/archive"
	"github.com/outofforest/build/v2/pkg/types"
	"github.com/outofforest/logger"
)

// Name is the type used for defining tool names.
type Name string

// Platform defines platform to install tool on.
type Platform struct {
	OS   string
	Arch string
}

func (p Platform) String() string {
	return p.OS + "." + p.Arch
}

// Platform constants.
const (
	OSLinux  = "linux"
	OSDarwin = "darwin"
	OSDocker = "docker"

	ArchAMD64 = "amd64"
	ArchARM64 = "arm64"
)

// Platform definitions.
var (
	PlatformLocal       = Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
	PlatformLinuxAMD64  = Platform{OS: OSLinux, Arch: ArchAMD64}
	PlatformDarwinAMD64 = Platform{OS: OSDarwin, Arch: ArchAMD64}
	PlatformDarwinARM64 = Platform{OS: OSDarwin, Arch: ArchARM64}
	PlatformDocker      = Platform{OS: OSDocker, Arch: runtime.GOARCH}
	PlatformDockerAMD64 = Platform{OS: OSDocker, Arch: ArchAMD64}
	PlatformDockerARM64 = Platform{OS: OSDocker, Arch: ArchARM64}
)

// Tool represents a tool to be installed.
type Tool interface {
	GetName() Name
	GetVersion() string
	IsCompatible(platform Platform) (bool, error)
	Verify(ctx context.Context) ([]error, error)
	Ensure(ctx context.Context, platform Platform) error
}

var toolsMap = map[Name]Tool{}

// Add adds tools to the toolset.
func Add(tools ...Tool) {
	for _, tool := range tools {
		toolsMap[tool.GetName()] = tool
	}
}

// Source represents source where tool is fetched from.
type Source struct {
	URL   string
	Hash  string
	Links map[string]string
}

// Sources is the map of sources.
type Sources map[Platform]Source

// BinaryTool is the tool having compiled binaries available on the internet.
type BinaryTool struct {
	Name    Name
	Version string
	Sources Sources
}

// GetName returns the anme of the tool.
func (bt BinaryTool) GetName() Name {
	return bt.Name
}

// GetVersion returns the version of the tool.
func (bt BinaryTool) GetVersion() string {
	return bt.Version
}

// IsCompatible checks if tool is compatible with the platform.
func (bt BinaryTool) IsCompatible(platform Platform) (bool, error) {
	_, exists := bt.Sources[platform]
	return exists, nil
}

// Verify verifies the cheksums.
func (bt BinaryTool) Verify(ctx context.Context) ([]error, error) {
	errs := []error{}
	for platform, source := range bt.Sources {
		resp, err := http.DefaultClient.Do(lo.Must(http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)))
		if err != nil {
			return nil, errors.WithStack(err)
		}
		defer resp.Body.Close()

		reader, err := archive.NewHashingReader(resp.Body, source.Hash)
		if err != nil {
			return nil, errors.Wrapf(err, "crearting hasher failed for tool %s and platform %s, url: %s",
				bt.Name, platform, source.URL)
		}
		if err := reader.ValidateChecksum(); err != nil {
			errs = append(errs, errors.Wrapf(err, "checksum does not match for tool %s and platform %s, url: %s",
				bt.Name, platform, source.URL))
		}
	}
	return errs, nil
}

// Ensure ensures the tool is installed.
func (bt BinaryTool) Ensure(ctx context.Context, platform Platform) error {
	source, exists := bt.Sources[platform]
	if !exists {
		return errors.Errorf("tool %s is not configured for platform %s", bt.Name, platform)
	}

	var install bool
	for dst, src := range source.Links {
		if ShouldReinstall(ctx, platform, bt, dst, src) {
			install = true
			break
		}
	}

	if install {
		if err := bt.install(ctx, platform); err != nil {
			return err
		}
	}

	return LinkFiles(ctx, platform, bt, lo.Keys(lo.Assign(source.Links)))
}

func (bt BinaryTool) install(ctx context.Context, platform Platform) (retErr error) {
	source, exists := bt.Sources[platform]
	if !exists {
		panic(errors.Errorf("tool %s is not configured for platform %s", bt.Name, platform))
	}

	ctx = logger.With(ctx,
		zap.String("tool", string(bt.Name)),
		zap.String("version", bt.Version),
		zap.String("url", source.URL),
		zap.Stringer("platform", platform))
	log := logger.Get(ctx)
	log.Info("Installing binaries")

	resp, err := http.DefaultClient.Do(lo.Must(http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)))
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close()

	reader, err := archive.NewHashingReader(resp.Body, source.Hash)
	if err != nil {
		return errors.Wrapf(err, "crearting hasher failed for tool %s and platform %s, url: %s",
			bt.Name, platform, source.URL)
	}

	downloadDir := ToolDownloadDir(ctx, platform, bt)
	if err := os.RemoveAll(downloadDir); err != nil && !os.IsNotExist(err) {
		return err
	}

	err = archive.Inflate(source.URL, reader, downloadDir)
	switch {
	case err == nil:
	case errors.Is(err, archive.ErrUnknownArchiveFormat):
		f, err := os.OpenFile(filepath.Join(downloadDir, filepath.Base(source.URL)),
			os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
		if err != nil {
			return errors.WithStack(err)
		}
		defer f.Close()
		if _, err := io.Copy(f, reader); err != nil {
			return errors.WithStack(err)
		}
	default:
		return err
	}

	if err := reader.ValidateChecksum(); err != nil {
		return errors.Wrapf(err, "checksum does not match for tool %s, url: %s", bt.Name, source.URL)
	}

	linksDir := ToolLinksDir(ctx, platform, bt)
	for dst, src := range source.Links {
		srcPath := filepath.Join(downloadDir, src)

		binChecksum, err := Checksum(srcPath)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(linksDir, dst)
		dstPathChecksum := dstPath + ":" + binChecksum
		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}
		if err := os.Remove(dstPathChecksum); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
			return errors.WithStack(err)
		}

		if err := os.Chmod(srcPath, 0o700); err != nil {
			return errors.WithStack(err)
		}
		srcLinkPath, err := filepath.Rel(filepath.Dir(dstPathChecksum), filepath.Join(downloadDir, src))
		if err != nil {
			return errors.WithStack(err)
		}
		if err := os.Symlink(srcLinkPath, dstPathChecksum); err != nil {
			return errors.WithStack(err)
		}
		if err := os.Symlink(filepath.Base(dstPathChecksum), dstPath); err != nil {
			return errors.WithStack(err)
		}

		log.Info("Binary installed to path", zap.String("path", dstPath))
	}

	log.Info("Binaries installed")
	return nil
}

// EnsureAll ensures all the tools.
func EnsureAll(ctx context.Context, _ types.DepsFunc) error {
	for _, tool := range toolsMap {
		isCompatible, err := tool.IsCompatible(PlatformLocal)
		if err != nil {
			return err
		}
		if !isCompatible {
			continue
		}
		if err := tool.Ensure(ctx, PlatformLocal); err != nil {
			return err
		}
	}
	return nil
}

// Ensure ensures tool exists for the platform.
func Ensure(ctx context.Context, toolName Name, platform Platform) error {
	tool, err := Get(toolName)
	if err != nil {
		return err
	}
	return tool.Ensure(ctx, platform)
}

// VerifyChecksums of all the tools.
func VerifyChecksums(ctx context.Context, _ types.DepsFunc) error {
	allErrs := []error{}
	for _, tool := range toolsMap {
		errs, err := tool.Verify(ctx)
		if err != nil {
			return err
		}
		allErrs = append(allErrs, errs...)
	}
	return goerrors.Join(allErrs...)
}

// VersionDir returns path to the version directory.
func VersionDir(ctx context.Context, platform Platform) string {
	return filepath.Join(PlatformDir(ctx, platform), GetVersion(ctx))
}

// Bin returns path to the installed binary.
func Bin(ctx context.Context, binary string, platform Platform) string {
	return lo.Must(filepath.Abs(lo.Must(filepath.EvalSymlinks(
		filepath.Join(VersionDir(ctx, platform), binary)))))
}

// Get returns the tool.
func Get(toolName Name) (Tool, error) {
	t, exists := toolsMap[toolName]
	if !exists {
		return nil, errors.Errorf("tool %s does not exist", toolName)
	}
	return t, nil
}

// EnvDir returns the directory where local environment is stored.
func EnvDir(ctx context.Context) string {
	return filepath.Join(lo.Must(os.UserCacheDir()), GetName(ctx))
}

// PlatformDir returns the directory where platform-specific stuff is stored.
func PlatformDir(ctx context.Context, platform Platform) string {
	return filepath.Join(EnvDir(ctx), platform.String())
}

// ToolDownloadDir returns directory where tool is downloaded.
func ToolDownloadDir(ctx context.Context, platform Platform, tool Tool) string {
	return filepath.Join(downloadsDir(ctx, platform), string(tool.GetName())+"-"+tool.GetVersion())
}

// ToolLinksDir returns directory where tools should be linked.
func ToolLinksDir(ctx context.Context, platform Platform, tool Tool) string {
	return filepath.Join(ToolDownloadDir(ctx, platform, tool), "_links")
}

// DevDir returns directory where development files are stored.
func DevDir(ctx context.Context) string {
	return filepath.Join(EnvDir(ctx), "dev")
}

// ShouldReinstall check if tool should be reinstalled due to missing files or links.
func ShouldReinstall(ctx context.Context, platform Platform, tool Tool, dst, src string) bool {
	srcAbsPath, err := filepath.Abs(filepath.Join(ToolDownloadDir(ctx, platform, tool), src))
	if err != nil {
		return true
	}

	srcRealPath, err := filepath.EvalSymlinks(srcAbsPath)
	if err != nil {
		return true
	}

	dstAbsPath, err := filepath.Abs(filepath.Join(ToolLinksDir(ctx, platform, tool), dst))
	if err != nil {
		return true
	}

	dstRealPath, err := filepath.EvalSymlinks(dstAbsPath)
	if err != nil || dstRealPath != srcRealPath {
		return true
	}

	fInfo, err := os.Stat(dstRealPath)
	if err != nil {
		return true
	}
	if fInfo.Mode()&0o700 == 0 {
		return true
	}

	linkedPath, err := os.Readlink(dstAbsPath)
	if err != nil {
		return true
	}
	linkNameParts := strings.Split(filepath.Base(linkedPath), ":")
	if len(linkNameParts) < 3 {
		return true
	}

	f, err := os.Open(dstRealPath)
	if err != nil {
		return true
	}
	defer f.Close()

	reader, err := archive.NewHashingReader(f,
		linkNameParts[len(linkNameParts)-2]+":"+linkNameParts[len(linkNameParts)-1])
	return err != nil || reader.ValidateChecksum() != nil
}

// LinkFiles creates all the links for the tool.
func LinkFiles(ctx context.Context, platform Platform, tool Tool, binaries []string) error {
	for _, dst := range binaries {
		relink, err := shouldRelinkFile(ctx, platform, tool, dst)
		if err != nil {
			return err
		}

		if !relink {
			continue
		}

		dstVersion := filepath.Join(VersionDir(ctx, platform), dst)
		src, err := filepath.Rel(filepath.Dir(dstVersion), filepath.Join(ToolLinksDir(ctx, platform, tool), dst))
		if err != nil {
			return errors.WithStack(err)
		}

		if err := os.Remove(dstVersion); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.WithStack(err)
		}

		if err := os.MkdirAll(filepath.Dir(dstVersion), 0o700); err != nil {
			return errors.WithStack(err)
		}

		if err := os.Symlink(src, dstVersion); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

// Checksum computes the checksum of a file.
func Checksum(file string) (string, error) {
	f, err := os.OpenFile(file, os.O_RDONLY, 0o600)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", errors.WithStack(err)
	}

	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func downloadsDir(ctx context.Context, platform Platform) string {
	return filepath.Join(PlatformDir(ctx, platform), "downloads")
}

func shouldRelinkFile(ctx context.Context, platform Platform, tool Tool, dst string) (bool, error) {
	srcPath := filepath.Join(ToolLinksDir(ctx, platform, tool), dst)

	realSrcPath, err := filepath.EvalSymlinks(srcPath)
	if err != nil {
		return false, errors.WithStack(err)
	}

	versionedPath := filepath.Join(VersionDir(ctx, platform), dst)
	realVersionedPath, err := filepath.EvalSymlinks(versionedPath)
	if err != nil {
		return true, nil //nolint:nilerr // this is ok
	}

	return realSrcPath != realVersionedPath, nil
}
