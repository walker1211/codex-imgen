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
	AgentsInstallDir   string

	claudeInstallParent   string
	openClawInstallParent string
	agentsInstallParent   string
	syncClaude            bool
	syncOpenClaw          bool
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
	claudeRoot := filepath.Join(home, ".claude")
	claudeInstallParent := filepath.Join(claudeRoot, "skills")
	openClawRoot := filepath.Join(home, ".openclaw")
	openClawInstallParent := filepath.Join(openClawRoot, "workspace", "skills")
	agentsInstallParent := filepath.Join(home, ".agents", "skills")
	return Paths{
		RepoRoot:              repoRoot,
		ClaudeInstallDir:      filepath.Join(claudeInstallParent, "imgen"),
		OpenClawInstallDir:    filepath.Join(openClawInstallParent, "imgen"),
		AgentsInstallDir:      filepath.Join(agentsInstallParent, "imgen"),
		claudeInstallParent:   claudeInstallParent,
		openClawInstallParent: openClawInstallParent,
		agentsInstallParent:   agentsInstallParent,
		syncClaude:            dirExists(claudeRoot),
		syncOpenClaw:          dirExists(openClawRoot),
	}
}

func (p Paths) claudeParent() string {
	return p.claudeInstallParent
}

func (p Paths) openClawParent() string {
	return p.openClawInstallParent
}

func (p Paths) agentsParent() string {
	return p.agentsInstallParent
}

func (p Paths) WithClaudeInstallDir(path string) Paths {
	p.ClaudeInstallDir = path
	p.claudeInstallParent = filepath.Dir(filepath.Clean(path))
	p.syncClaude = true
	return p
}

func (p Paths) WithOpenClawInstallDir(path string) Paths {
	p.OpenClawInstallDir = path
	p.openClawInstallParent = filepath.Dir(filepath.Clean(path))
	p.syncOpenClaw = true
	return p
}

func (p Paths) WithAgentsInstallDir(path string) Paths {
	p.AgentsInstallDir = path
	p.agentsInstallParent = filepath.Dir(filepath.Clean(path))
	return p
}

// WithCodexInstallDir is kept as a compatibility alias for callers that used
// the previous Codex-specific name before personal skills moved to .agents.
func (p Paths) WithCodexInstallDir(path string) Paths {
	return p.WithAgentsInstallDir(path)
}

func (p Paths) Pairs() []Pair {
	sourceDir := p.agentsSourceDir()
	pairs := []Pair{
		{
			Name:           "agents",
			SourceDir:      sourceDir,
			DestinationDir: p.AgentsInstallDir,
			InstallParent:  p.agentsParent(),
		},
	}
	if p.syncOpenClaw {
		pairs = append(pairs, Pair{
			Name:           "openclaw",
			SourceDir:      sourceDir,
			DestinationDir: p.OpenClawInstallDir,
			InstallParent:  p.openClawParent(),
		})
	}
	if p.syncClaude {
		pairs = append(pairs, Pair{
			Name:           "claude",
			SourceDir:      sourceDir,
			DestinationDir: p.ClaudeInstallDir,
			InstallParent:  p.claudeParent(),
		})
	}
	return pairs
}

func (p Paths) agentsSourceDir() string {
	return filepath.Join(p.RepoRoot, ".agents", "skills", "imgen")
}

func (p Paths) openClawRepositoryDir() string {
	return filepath.Join(p.RepoRoot, ".openclaw", "skills", "imgen")
}

func (p Paths) Check() (Result, error) {
	var result Result
	sourceDir := p.agentsSourceDir()
	if err := validateSource(sourceDir); err != nil {
		return Result{}, fmt.Errorf("agents source invalid: %w", err)
	}
	pairs := p.Pairs()
	for _, pair := range pairs {
		drift, err := comparePair(pair)
		if err != nil {
			return Result{}, err
		}
		result.Drift = append(result.Drift, drift...)
	}
	drift, err := compareRepositoryMirror(sourceDir, p.openClawRepositoryDir())
	if err != nil {
		return Result{}, err
	}
	result.Drift = append(result.Drift, drift...)
	sort.Strings(result.Drift)
	return result, nil
}

func (p Paths) Apply() (Result, error) {
	var result Result
	sourceDir := p.agentsSourceDir()
	if err := validateSource(sourceDir); err != nil {
		return Result{}, fmt.Errorf("agents source invalid: %w", err)
	}
	pairs := p.Pairs()
	for _, pair := range pairs {
		if err := validateDestination(pair.DestinationDir, pair.InstallParent); err != nil {
			return Result{}, fmt.Errorf("%s destination invalid: %w", pair.Name, err)
		}
	}
	openClawRepositoryDir := p.openClawRepositoryDir()
	if err := os.RemoveAll(openClawRepositoryDir); err != nil {
		return Result{}, fmt.Errorf("remove openclaw repository skill: %w", err)
	}
	if err := copyDir(sourceDir, openClawRepositoryDir); err != nil {
		return Result{}, fmt.Errorf("copy openclaw repository skill: %w", err)
	}
	result.Applied = append(result.Applied, openClawRepositoryDir)
	for _, pair := range pairs {
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
		hasSkillSource := dirExists(filepath.Join(dir, ".agents", "skills", "imgen"))
		hasProjectMarker := fileExists(filepath.Join(dir, "go.mod")) ||
			fileExists(filepath.Join(dir, "configs", "config.example.yaml")) ||
			fileExists(filepath.Join(dir, "configs", "config.yaml"))
		if hasSkillSource && hasProjectMarker {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find codex-imgen repository root from %s", cwd)
		}
		dir = parent
	}
}

func CompareSkillTrees(sourceDir string, destinationDir string, label string) ([]string, error) {
	return compareSkillTrees(sourceDir, destinationDir, label, label)
}

func compareSkillTrees(sourceDir string, destinationDir string, label string, missingLabel string) ([]string, error) {
	sourceFiles, err := listFiles(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("read source skill: %w", err)
	}
	destinationFiles, err := listFiles(destinationDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{fmt.Sprintf("%s missing: %s", missingLabel, destinationDir)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}

	var drift []string
	for rel, sourcePath := range sourceFiles {
		destinationPath, ok := destinationFiles[rel]
		if !ok {
			drift = append(drift, fmt.Sprintf("%s file missing: %s", label, rel))
			continue
		}
		same, err := sameFile(sourcePath, destinationPath)
		if err != nil {
			return nil, err
		}
		if !same {
			drift = append(drift, fmt.Sprintf("%s file differs: %s", label, rel))
		}
	}
	for rel := range destinationFiles {
		if _, ok := sourceFiles[rel]; !ok {
			drift = append(drift, fmt.Sprintf("%s extra file: %s", label, rel))
		}
	}
	sort.Strings(drift)
	return drift, nil
}

func compareRepositoryMirror(sourceDir string, mirrorDir string) ([]string, error) {
	return compareSkillTrees(sourceDir, mirrorDir, "repo openclaw", "repo openclaw skill")
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
