//go:build ignore

// Command release builds and packages every supported zot release target.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type target struct {
	goos   string
	goarch string
	format string
}

var releaseTargets = []target{
	{goos: "linux", goarch: "amd64", format: "tar.gz"},
	{goos: "linux", goarch: "arm64", format: "tar.gz"},
	{goos: "windows", goarch: "amd64", format: "zip"},
	{goos: "darwin", goarch: "amd64", format: "tar.gz"},
	{goos: "darwin", goarch: "arm64", format: "tar.gz"},
}

var safeMetadata = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+\-]*$`)

func main() {
	version := flag.String("version", "", "release version used in binaries and artifact names")
	commit := flag.String("commit", "unknown", "Git commit embedded in binaries")
	buildDate := flag.String("build-date", "", "UTC build date embedded in binaries")
	dist := flag.String("dist", "dist", "output directory")
	upx := flag.String("upx", "upx", "UPX executable")
	flag.Parse()

	if err := run(*version, *commit, *buildDate, *dist, *upx); err != nil {
		fmt.Fprintf(os.Stderr, "release: %v\n", err)
		os.Exit(1)
	}
}

func run(version, commit, buildDate, dist, upx string) error {
	if !safeMetadata.MatchString(version) {
		return fmt.Errorf("invalid version %q", version)
	}
	if !safeMetadata.MatchString(commit) {
		return fmt.Errorf("invalid commit %q", commit)
	}
	if strings.TrimSpace(buildDate) == "" || strings.ContainsAny(buildDate, "\r\n") {
		return fmt.Errorf("invalid build date %q", buildDate)
	}
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("Go toolchain is required: %w", err)
	}
	upxPath, err := exec.LookPath(upx)
	if err != nil {
		return fmt.Errorf("UPX is required for Linux and Windows release binaries: %w", err)
	}

	distAbs, err := safeDistPath(dist)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(distAbs); err != nil {
		return fmt.Errorf("clean dist directory: %w", err)
	}
	if err := os.MkdirAll(distAbs, 0o755); err != nil {
		return fmt.Errorf("create dist directory: %w", err)
	}

	stageRoot := filepath.Join(distAbs, ".package")
	defer os.RemoveAll(stageRoot)
	ldflags := strings.Join([]string{
		"-s", "-w",
		"-X", "zotero_cli/internal/cli.version=" + version,
		"-X", "zotero_cli/internal/cli.commit=" + commit,
		"-X", "zotero_cli/internal/cli.buildDate=" + buildDate,
	}, " ")

	for _, target := range releaseTargets {
		if err := packageTarget(target, version, ldflags, distAbs, stageRoot, upxPath); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(stageRoot); err != nil {
		return fmt.Errorf("remove staging directory: %w", err)
	}
	if err := writeChecksums(distAbs); err != nil {
		return err
	}

	fmt.Printf("release artifacts written to %s\n", distAbs)
	return listArtifacts(distAbs)
}

func safeDistPath(dist string) (string, error) {
	if strings.TrimSpace(dist) == "" {
		return "", fmt.Errorf("dist directory must not be empty")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	abs, err := filepath.Abs(dist)
	if err != nil {
		return "", fmt.Errorf("resolve dist directory: %w", err)
	}
	rel, err := filepath.Rel(cwd, abs)
	if err != nil {
		return "", fmt.Errorf("validate dist directory: %w", err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("dist directory must be inside the repository: %s", abs)
	}
	return abs, nil
}

func packageTarget(t target, version, ldflags, dist, stageRoot, upx string) error {
	artifactBase := fmt.Sprintf("zot_%s_%s_%s", version, t.goos, t.goarch)
	packageDir := filepath.Join(stageRoot, artifactBase)
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		return fmt.Errorf("create package directory for %s/%s: %w", t.goos, t.goarch, err)
	}

	ext := ""
	if t.goos == "windows" {
		ext = ".exe"
	}
	binary := filepath.Join(packageDir, "zot"+ext)
	fmt.Printf("building %s/%s\n", t.goos, t.goarch)
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", binary, "./cmd/zot")
	cmd.Env = append(os.Environ(), "GOOS="+t.goos, "GOARCH="+t.goarch, "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build %s/%s: %w", t.goos, t.goarch, err)
	}

	if t.goos == "linux" || t.goos == "windows" {
		if err := compressUPX(upx, binary); err != nil {
			return fmt.Errorf("compress %s/%s: %w", t.goos, t.goarch, err)
		}
	}
	for _, name := range []string{"README.md", "LICENSE"} {
		if err := copyFile(name, filepath.Join(packageDir, name)); err != nil {
			return fmt.Errorf("copy %s into %s/%s package: %w", name, t.goos, t.goarch, err)
		}
	}

	if t.goos == "windows" {
		if err := copyFile(binary, filepath.Join(dist, artifactBase+".exe")); err != nil {
			return fmt.Errorf("write standalone Windows executable: %w", err)
		}
	}

	archive := filepath.Join(dist, artifactBase+"."+t.format)
	switch t.format {
	case "zip":
		if err := writeZip(archive, stageRoot, artifactBase); err != nil {
			return fmt.Errorf("package %s: %w", artifactBase, err)
		}
	case "tar.gz":
		if err := writeTarGz(archive, stageRoot, artifactBase); err != nil {
			return fmt.Errorf("package %s: %w", artifactBase, err)
		}
	default:
		return fmt.Errorf("unsupported archive format %q", t.format)
	}
	return os.RemoveAll(packageDir)
}

func compressUPX(upx, binary string) error {
	temporary := binary + ".tmp"
	defer os.Remove(temporary)
	cmd := exec.Command(upx, "--best", "--lzma", "-o", temporary, binary)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	if err := os.Remove(binary); err != nil {
		return err
	}
	if err := os.Rename(temporary, binary); err != nil {
		return err
	}
	test := exec.Command(upx, "-t", binary)
	test.Stdout = os.Stdout
	test.Stderr = os.Stderr
	return test.Run()
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func writeZip(destination, root, packageName string) error {
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(file)
	walkErr := filepath.WalkDir(filepath.Join(root, packageName), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		return copyInto(writer, path)
	})
	closeErr := zw.Close()
	fileErr := file.Close()
	if walkErr != nil {
		return walkErr
	}
	if closeErr != nil {
		return closeErr
	}
	return fileErr
}

func writeTarGz(destination, root, packageName string) error {
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	walkErr := filepath.WalkDir(filepath.Join(root, packageName), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if entry.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		return copyInto(tw, path)
	})
	twErr := tw.Close()
	gzErr := gz.Close()
	fileErr := file.Close()
	if walkErr != nil {
		return walkErr
	}
	if twErr != nil {
		return twErr
	}
	if gzErr != nil {
		return gzErr
	}
	return fileErr
}

func copyInto(destination io.Writer, source string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(destination, file)
	return err
}

func writeChecksums(dist string) (returnErr error) {
	entries, err := os.ReadDir(dist)
	if err != nil {
		return fmt.Errorf("read dist directory: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "checksums.txt" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	checksums, err := os.Create(filepath.Join(dist, "checksums.txt"))
	if err != nil {
		return fmt.Errorf("create checksums.txt: %w", err)
	}
	defer func() {
		if err := checksums.Close(); returnErr == nil {
			returnErr = err
		}
	}()
	for _, name := range names {
		file, err := os.Open(filepath.Join(dist, name))
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if _, err := fmt.Fprintf(checksums, "%x  %s\n", hash.Sum(nil), name); err != nil {
			return err
		}
	}
	return nil
}

func listArtifacts(dist string) error {
	entries, err := os.ReadDir(dist)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Printf("  %-48s %s\n", entry.Name(), humanBytes(info.Size()))
	}
	return nil
}

func humanBytes(size int64) string {
	const unit = int64(1024)
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KiB", "MiB", "GiB"}
	for _, suffix := range units {
		value /= float64(unit)
		if value < float64(unit) || suffix == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%d B", size)
}
