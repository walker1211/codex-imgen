package skillsync

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Paths struct {
	RepoRoot           string
	ClaudeInstallDir   string
	OpenClawInstallDir string

	claudeInstallParent   string
	openClawInstallParent string
}

type Pair struct {
	Name           string
	SourceDir      string
	DestinationDir string
	InstallParent  string
}

type Result struct {
	Drift   []string
	Applied []string
}

func DefaultPaths(repoRoot string, home string) Paths {
	claudeInstallParent := filepath.Join(home, ".claude", "skills")
	openClawInstallParent := filepath.Join(home, ".openclaw", "workspace", "skills")
	return Paths{
		RepoRoot:              repoRoot,
		ClaudeInstallDir:      filepath.Join(claudeInstallParent, "imgen"),
		OpenClawInstallDir:    filepath.Join(openClawInstallParent, "imgen"),
		claudeInstallParent:   claudeInstallParent,
		openClawInstallParent: openClawInstallParent,
	}
}

func (p Paths) claudeParent() string {
	return p.claudeInstallParent
}

func (p Paths) openClawParent() string {
	return p.openClawInstallParent
}

func (p Paths) WithClaudeInstallDir(path string) Paths {
	p.ClaudeInstallDir = path
	p.claudeInstallParent = filepath.Dir(filepath.Clean(path))
	return p
}

func (p Paths) WithOpenClawInstallDir(path string) Paths {
	p.OpenClawInstallDir = path
	p.openClawInstallParent = filepath.Dir(filepath.Clean(path))
	return p
}

func (p Paths) Pairs() []Pair {
	return []Pair{
		{
			Name:           "claude",
			SourceDir:      filepath.Join(p.RepoRoot, ".claude", "skills", "imgen"),
			DestinationDir: p.ClaudeInstallDir,
			InstallParent:  p.claudeParent(),
		},
		{
			Name:           "openclaw",
			SourceDir:      filepath.Join(p.RepoRoot, ".openclaw", "skills", "imgen"),
			DestinationDir: p.OpenClawInstallDir,
			InstallParent:  p.openClawParent(),
		},
	}
}

func (p Paths) Check() (Result, error) {
	var result Result
	pairs := p.Pairs()
	for _, pair := range pairs {
		if err := validateSource(pair.SourceDir); err != nil {
			return Result{}, fmt.Errorf("%s source invalid: %w", pair.Name, err)
		}
		drift, err := comparePair(pair)
		if err != nil {
			return Result{}, err
		}
		result.Drift = append(result.Drift, drift...)
	}
	drift, err := compareRepoReferences(pairs[0].SourceDir, pairs[1].SourceDir)
	if err != nil {
		return Result{}, err
	}
	result.Drift = append(result.Drift, drift...)
	sort.Strings(result.Drift)
	return result, nil
}

func (p Paths) Apply() (Result, error) {
	var result Result
	for _, pair := range p.Pairs() {
		if err := validateSource(pair.SourceDir); err != nil {
			return Result{}, fmt.Errorf("%s source invalid: %w", pair.Name, err)
		}
		if err := validateDestination(pair.DestinationDir, pair.InstallParent); err != nil {
			return Result{}, fmt.Errorf("%s destination invalid: %w", pair.Name, err)
		}
		if err := os.RemoveAll(pair.DestinationDir); err != nil {
			return Result{}, fmt.Errorf("remove %s install: %w", pair.Name, err)
		}
		if err := copyDir(pair.SourceDir, pair.DestinationDir); err != nil {
			return Result{}, fmt.Errorf("copy %s skill: %w", pair.Name, err)
		}
		result.Applied = append(result.Applied, pair.DestinationDir)
	}
	return result, nil
}

func FindRepositoryRoot(cwd string) (string, error) {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	for {
		if fileExists(filepath.Join(dir, "go.mod")) && dirExists(filepath.Join(dir, ".claude", "skills", "imgen")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find codex-imgen repository root from %s", cwd)
		}
		dir = parent
	}
}

func compareRepoReferences(claudeSourceDir string, openClawSourceDir string) ([]string, error) {
	claudeReferences, err := listFiles(filepath.Join(claudeSourceDir, "references"))
	if err != nil {
		return nil, fmt.Errorf("read claude repository references: %w", err)
	}
	openClawReferences, err := listFiles(filepath.Join(openClawSourceDir, "references"))
	if err != nil {
		return nil, fmt.Errorf("read openclaw repository references: %w", err)
	}

	var drift []string
	for rel, claudePath := range claudeReferences {
		openClawPath, ok := openClawReferences[rel]
		if !ok {
			drift = append(drift, fmt.Sprintf("repo openclaw reference missing: references/%s", rel))
			continue
		}
		same, err := sameFile(claudePath, openClawPath)
		if err != nil {
			return nil, err
		}
		if !same {
			drift = append(drift, fmt.Sprintf("repo reference differs: references/%s", rel))
		}
	}
	for rel := range openClawReferences {
		if _, ok := claudeReferences[rel]; !ok {
			drift = append(drift, fmt.Sprintf("repo openclaw extra reference: references/%s", rel))
		}
	}
	sort.Strings(drift)
	return drift, nil
}

func comparePair(pair Pair) ([]string, error) {
	sourceFiles, err := listFiles(pair.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("read %s source: %w", pair.Name, err)
	}
	destinationFiles, err := listFilesWithSymlinks(pair.DestinationDir, true)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{fmt.Sprintf("%s install missing: %s", pair.Name, pair.DestinationDir)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s install: %w", pair.Name, err)
	}

	var drift []string
	for rel, sourcePath := range sourceFiles {
		destinationPath, ok := destinationFiles[rel]
		if !ok {
			drift = append(drift, fmt.Sprintf("%s file missing: %s", pair.Name, rel))
			continue
		}
		isSymlink, err := isSymlink(destinationPath)
		if err != nil {
			return nil, err
		}
		if isSymlink {
			drift = append(drift, fmt.Sprintf("%s file is symlink: %s", pair.Name, rel))
			continue
		}
		same, err := sameFile(sourcePath, destinationPath)
		if err != nil {
			return nil, err
		}
		if !same {
			drift = append(drift, fmt.Sprintf("%s file differs: %s", pair.Name, rel))
		}
	}
	for rel := range destinationFiles {
		if _, ok := sourceFiles[rel]; !ok {
			drift = append(drift, fmt.Sprintf("%s extra install file: %s", pair.Name, rel))
		}
	}
	sort.Strings(drift)
	return drift, nil
}

func validateSource(root string) error {
	files, err := listFiles(root)
	if err != nil {
		return err
	}
	markers := []string{"T" + "BD", "TO" + "DO", "PLACE" + "HOLDER"}
	secretAssignments := []string{"EMAIL_SMTP_AUTH_CODE=", "ANTHROPIC_API_KEY=", "OPENAI_API_KEY="}
	for rel, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)
		for _, marker := range markers {
			if strings.Contains(text, marker) {
				return fmt.Errorf("%s contains unresolved marker %q", rel, marker)
			}
		}
		for _, assignment := range secretAssignments {
			if strings.Contains(text, assignment) {
				return fmt.Errorf("%s contains secret assignment %q", rel, assignment)
			}
		}
	}
	return nil
}

func validateDestination(path string, installParent string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("destination must be absolute: %s", path)
	}
	if !filepath.IsAbs(installParent) {
		return fmt.Errorf("expected install parent must be absolute: %s", installParent)
	}
	clean := filepath.Clean(path)
	cleanParent := filepath.Clean(installParent)
	if filepath.Base(clean) != "imgen" {
		return fmt.Errorf("destination must end with imgen: %s", path)
	}
	if filepath.Base(filepath.Dir(clean)) != "skills" {
		return fmt.Errorf("destination parent must be skills: %s", path)
	}
	rel, err := filepath.Rel(cleanParent, clean)
	if err != nil {
		return err
	}
	if rel != "imgen" {
		return fmt.Errorf("destination outside expected install parent %s: %s", cleanParent, path)
	}
	return nil
}

func listFiles(root string) (map[string]string, error) {
	return listFilesWithSymlinks(root, false)
}

func listFilesWithSymlinks(root string, allowSymlinks bool) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&fs.ModeSymlink != 0 && !allowSymlinks {
			return fmt.Errorf("source contains symlink: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = path
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func copyDir(source string, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("source contains symlink: %s", path)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, info.Mode().Perm())
	})
}

func isSymlink(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return info.Mode()&fs.ModeSymlink != 0, nil
}

func sameFile(left string, right string) (bool, error) {
	leftContent, err := os.ReadFile(left)
	if err != nil {
		return false, err
	}
	rightContent, err := os.ReadFile(right)
	if err != nil {
		return false, err
	}
	return bytes.Equal(leftContent, rightContent), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
