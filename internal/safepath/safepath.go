package safepath

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// JoinRelative joins a portable slash-separated relative path below root.
// Both slash styles are treated as separators so a manifest produced on one
// platform cannot become unsafe when consumed on another.
func JoinRelative(root, relative string) (string, error) {
	portable := strings.ReplaceAll(strings.TrimSpace(relative), `\`, "/")
	if portable == "" || strings.HasPrefix(portable, "/") || hasDrivePrefix(portable) {
		return "", fmt.Errorf("path %q is not a safe relative path", relative)
	}
	clean := path.Clean(portable)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path %q escapes its root", relative)
	}
	joined := filepath.Join(root, filepath.FromSlash(clean))
	if !Within(root, joined) {
		return "", fmt.Errorf("path %q escapes its root", relative)
	}
	return joined, nil
}

// JoinComponents joins path components that must each be a single portable
// filename. It is suitable for attachment keys and filenames from manifests.
func JoinComponents(root string, components ...string) (string, error) {
	for _, component := range components {
		if strings.TrimSpace(component) == "" || component == "." || component == ".." ||
			strings.ContainsAny(component, `/\`) || hasDrivePrefix(component) {
			return "", fmt.Errorf("path component %q is invalid", component)
		}
	}
	return JoinRelative(root, strings.Join(components, "/"))
}

// Within performs a lexical containment check after making both paths
// absolute. It does not follow symbolic links.
func Within(root, candidate string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(rootAbs), filepath.Clean(candidateAbs))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// ExistingRegularFileWithin verifies lexical and symlink-resolved containment
// and requires candidate to be a regular file.
func ExistingRegularFileWithin(root, candidate string) bool {
	resolvedRoot, resolvedCandidate, ok := resolveWithin(root, candidate)
	if !ok {
		return false
	}
	info, err := os.Stat(resolvedCandidate)
	return Within(resolvedRoot, resolvedCandidate) && err == nil && info.Mode().IsRegular()
}

// ExistingDirectoryWithin is the directory counterpart used before creating
// or replacing a file below a potentially pre-existing directory tree.
func ExistingDirectoryWithin(root, candidate string) bool {
	resolvedRoot, resolvedCandidate, ok := resolveWithin(root, candidate)
	if !ok {
		return false
	}
	info, err := os.Stat(resolvedCandidate)
	return Within(resolvedRoot, resolvedCandidate) && err == nil && info.IsDir()
}

func resolveWithin(root, candidate string) (string, string, bool) {
	if !Within(root, candidate) {
		return "", "", false
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", false
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil || !Within(resolvedRoot, resolvedCandidate) {
		return "", "", false
	}
	return resolvedRoot, resolvedCandidate, true
}

func hasDrivePrefix(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) && value[1] == ':'
}
