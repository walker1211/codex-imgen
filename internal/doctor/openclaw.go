package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/walker1211/codex-imgen/internal/skillsync"
)

type Level string

const (
	LevelOK   Level = "OK"
	LevelWarn Level = "WARN"
	LevelFail Level = "FAIL"
)

type Item struct {
	Level   Level
	Message string
}

type Report struct {
	Title string
	Items []Item
}

func (r Report) Failed() bool {
	for _, item := range r.Items {
		if item.Level == LevelFail {
			return true
		}
	}
	return false
}

func (r Report) Render() string {
	var b strings.Builder
	if r.Title != "" {
		b.WriteString(r.Title)
		b.WriteByte('\n')
	}
	for _, item := range r.Items {
		fmt.Fprintf(&b, "[%s] %s\n", item.Level, item.Message)
	}
	return b.String()
}

type CommandRunner func(context.Context, string, ...string) (string, error)

type OpenClawChecker struct {
	HomeDir    string
	RepoRoot   string
	LookPath   func(string) (string, error)
	RunCommand CommandRunner
	Getwd      func() (string, error)
}

func NewOpenClawChecker(homeDir string) OpenClawChecker {
	return OpenClawChecker{HomeDir: homeDir}
}

func (c OpenClawChecker) lookPath(name string) (string, error) {
	if c.LookPath != nil {
		return c.LookPath(name)
	}
	return exec.LookPath(name)
}

func (c OpenClawChecker) runCommand(ctx context.Context, name string, args ...string) (string, error) {
	if c.RunCommand != nil {
		return c.RunCommand(ctx, name, args...)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (c OpenClawChecker) getwd() (string, error) {
	if c.Getwd != nil {
		return c.Getwd()
	}
	return os.Getwd()
}

func (c OpenClawChecker) repositoryRoot(report *Report) (string, bool) {
	if c.RepoRoot != "" {
		return c.RepoRoot, true
	}
	cwd, err := c.getwd()
	if err != nil {
		report.Items = append(report.Items, Item{Level: LevelWarn, Message: "repository root not checked: current working directory unavailable"})
		return "", false
	}
	root, err := skillsync.FindRepositoryRoot(cwd)
	if err != nil {
		report.Items = append(report.Items, Item{Level: LevelWarn, Message: "repository root not checked: codex-imgen checkout not discoverable"})
		return "", false
	}
	return root, true
}

func (c OpenClawChecker) Check(ctx context.Context) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}

	home := c.HomeDir
	report := Report{Title: "OpenClaw doctor"}
	configPath := filepath.Join(home, ".openclaw", "openclaw.json")
	skillPath := filepath.Join(home, ".openclaw", "workspace", "skills", "imgen", "SKILL.md")

	root, ok := readOpenClawConfig(configPath, &report)
	if ok {
		checkImageGenerateDenied(root, &report)
		mainAgent, mainFound := checkMainAgent(root, &report)
		messageExposed := false
		if mainFound {
			messageExposed = checkMainAgentMessage(mainAgent, &report)
		}
		checkTelegramSilentReply(root, &report)
		if messageExposed && activeMemoryTargetsMain(root) {
			report.Items = append(report.Items, Item{Level: LevelWarn, Message: "active-memory targets main while main exposes message; OpenClaw may log a no-callable-tools warning in embedded memory runs"})
		}
	}
	checkOpenClawSkill(skillPath, &report)
	if repoRoot, ok := c.repositoryRoot(&report); ok {
		checkSkillSync(repoRoot, skillPath, &report)
	}
	checkOpenClawCLI(ctx, c, &report)

	return report, nil
}

func readOpenClawConfig(path string, report *Report) (map[string]any, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		report.Items = append(report.Items, Item{Level: LevelFail, Message: "config file: ~/.openclaw/openclaw.json not found or unreadable"})
		return nil, false
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		report.Items = append(report.Items, Item{Level: LevelFail, Message: "config file: ~/.openclaw/openclaw.json is not valid JSON"})
		return nil, false
	}
	report.Items = append(report.Items, Item{Level: LevelOK, Message: "config file: ~/.openclaw/openclaw.json"})
	return root, true
}

func checkImageGenerateDenied(root map[string]any, report *Report) {
	if containsString(arrayAt(root, "tools", "deny"), "image_generate") {
		report.Items = append(report.Items, Item{Level: LevelOK, Message: "image_generate denied in tools.deny"})
	} else {
		report.Items = append(report.Items, Item{Level: LevelFail, Message: "image_generate is not denied in tools.deny"})
	}

	if containsString(arrayAt(root, "gateway", "tools", "deny"), "image_generate") {
		report.Items = append(report.Items, Item{Level: LevelOK, Message: "image_generate denied in gateway.tools.deny"})
	} else {
		report.Items = append(report.Items, Item{Level: LevelFail, Message: "image_generate is not denied in gateway.tools.deny"})
	}
}

func checkMainAgent(root map[string]any, report *Report) (map[string]any, bool) {
	for _, value := range arrayAt(root, "agents", "list") {
		agent, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if stringAt(agent, "id") == "main" {
			report.Items = append(report.Items, Item{Level: LevelOK, Message: "main agent exists"})
			return agent, true
		}
	}
	report.Items = append(report.Items, Item{Level: LevelFail, Message: "main agent is missing from agents.list"})
	return nil, false
}

func checkMainAgentMessage(mainAgent map[string]any, report *Report) bool {
	if containsString(arrayAt(mainAgent, "tools", "alsoAllow"), "message") || containsString(arrayAt(mainAgent, "tools", "allow"), "message") {
		report.Items = append(report.Items, Item{Level: LevelOK, Message: "main agent exposes message"})
		return true
	}
	report.Items = append(report.Items, Item{Level: LevelFail, Message: "main agent does not expose message in tools.alsoAllow or tools.allow"})
	return false
}

func checkTelegramSilentReply(root map[string]any, report *Report) {
	paths := [][]string{
		{"surfaces", "telegram", "silentReply", "direct"},
		{"surfaces", "silentReply", "direct"},
		{"telegram", "silentReply", "direct"},
		{"silentReply", "direct"},
	}
	for _, path := range paths {
		if stringAt(root, path...) == "allow" {
			report.Items = append(report.Items, Item{Level: LevelOK, Message: "telegram direct NO_REPLY is silent"})
			return
		}
	}
	report.Items = append(report.Items, Item{Level: LevelFail, Message: "telegram direct NO_REPLY is not configured as silent"})
}

func checkOpenClawCLI(ctx context.Context, checker OpenClawChecker, report *Report) {
	path, err := checker.lookPath("openclaw")
	if err != nil {
		report.Items = append(report.Items, Item{Level: LevelWarn, Message: "openclaw CLI not found on PATH; message send --force-document support not verified"})
		return
	}
	report.Items = append(report.Items, Item{Level: LevelOK, Message: "openclaw CLI found: " + path})

	output, err := checker.runCommand(ctx, path, "message", "send", "--help")
	if err != nil {
		report.Items = append(report.Items, Item{Level: LevelWarn, Message: "openclaw message send --help failed; force-document support not verified"})
		return
	}
	if strings.Contains(output, "--force-document") {
		report.Items = append(report.Items, Item{Level: LevelOK, Message: "openclaw CLI message send supports --force-document"})
		return
	}
	report.Items = append(report.Items, Item{Level: LevelFail, Message: "openclaw CLI message send does not expose --force-document"})
}

func checkOpenClawSkill(path string, report *Report) {
	data, err := os.ReadFile(path)
	if err != nil {
		report.Items = append(report.Items, Item{Level: LevelFail, Message: "imgen skill is not installed at ~/.openclaw/workspace/skills/imgen/SKILL.md"})
		return
	}
	text := string(data)
	missing := missingMarkers(text, map[string][]string{
		"./imgen --json":           {"./imgen --json"},
		"image_generate":           {"image_generate"},
		"NO_REPLY":                 {"NO_REPLY"},
		"forceDocument/asDocument": {"forceDocument", "asDocument"},
	})
	if !hasSyncJSONSuccessContract(text) {
		missing = append(missing, "sync JSON success")
	}
	if len(missing) > 0 {
		report.Items = append(report.Items, Item{Level: LevelFail, Message: "imgen skill is installed but missing guidance markers: " + strings.Join(missing, ", ")})
		return
	}
	report.Items = append(report.Items, Item{Level: LevelOK, Message: "imgen skill installed for OpenClaw"})
}

func checkSkillSync(repoRoot string, installedOpenClawSkillPath string, report *Report) {
	sourceDir := filepath.Join(repoRoot, ".claude", "skills", "imgen")
	repositoryOpenClawDir := filepath.Join(repoRoot, ".openclaw", "skills", "imgen")

	drift, err := skillsync.CompareSkillTrees(sourceDir, repositoryOpenClawDir, "repository OpenClaw skill mirror")
	if err != nil {
		report.Items = append(report.Items, Item{Level: LevelFail, Message: "repository OpenClaw skill mirror check failed: " + err.Error()})
	} else if len(drift) > 0 {
		report.Items = append(report.Items, Item{Level: LevelFail, Message: "repository OpenClaw skill mirror drift: " + strings.Join(drift, "; ")})
	} else {
		report.Items = append(report.Items, Item{Level: LevelOK, Message: "repository OpenClaw skill mirror matches Claude source"})
	}

	installedOpenClawDir := filepath.Dir(installedOpenClawSkillPath)
	drift, err = skillsync.CompareSkillTrees(sourceDir, installedOpenClawDir, "installed OpenClaw imgen skill")
	if err != nil {
		report.Items = append(report.Items, Item{Level: LevelFail, Message: "installed OpenClaw imgen skill check failed: " + err.Error()})
	} else if len(drift) > 0 {
		report.Items = append(report.Items, Item{Level: LevelFail, Message: "installed OpenClaw imgen skill drift: " + strings.Join(drift, "; ")})
	} else {
		report.Items = append(report.Items, Item{Level: LevelOK, Message: "installed OpenClaw imgen skill matches Claude source"})
	}
}

func hasSyncJSONSuccessContract(text string) bool {
	hasOK := strings.Contains(text, "ok=true") || strings.Contains(text, "ok true") || strings.Contains(text, "ok: true") || strings.Contains(text, "\"ok\": true")
	hasPath := strings.Contains(text, "images[].path") || strings.Contains(text, "images[0].path") || strings.Contains(text, "images[*].path")
	hasDone := strings.Contains(text, "status=done") || strings.Contains(text, "status done") || strings.Contains(text, "status can be done") || strings.Contains(text, "\"status\": \"done\"")
	return hasOK && hasPath && hasDone
}

func missingMarkers(text string, markers map[string][]string) []string {
	var missing []string
	for name, alternatives := range markers {
		found := false
		for _, alternative := range alternatives {
			if strings.Contains(text, alternative) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func arrayAt(root map[string]any, path ...string) []any {
	value, ok := valueAt(root, path...).([]any)
	if !ok {
		return nil
	}
	return value
}

func stringAt(root map[string]any, path ...string) string {
	value, ok := valueAt(root, path...).(string)
	if !ok {
		return ""
	}
	return value
}

func valueAt(root map[string]any, path ...string) any {
	var current any = root
	for _, part := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}

func containsString(values []any, target string) bool {
	for _, value := range values {
		if s, ok := value.(string); ok && s == target {
			return true
		}
	}
	return false
}

func activeMemoryTargetsMain(root map[string]any) bool {
	for _, node := range activeMemoryNodes(root) {
		if containsStringValue(node, "main") {
			return true
		}
	}
	return false
}

func activeMemoryNodes(value any) []any {
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	var nodes []any
	for key, child := range object {
		if isActiveMemoryKey(key) {
			nodes = append(nodes, child)
			continue
		}
		nodes = append(nodes, activeMemoryNodes(child)...)
	}
	return nodes
}

func isActiveMemoryKey(key string) bool {
	normalized := strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(key))
	return normalized == "activememory" || strings.Contains(normalized, "activememory")
}

func containsStringValue(value any, target string) bool {
	switch v := value.(type) {
	case string:
		return v == target
	case []any:
		for _, item := range v {
			if containsStringValue(item, target) {
				return true
			}
		}
	case map[string]any:
		for _, item := range v {
			if containsStringValue(item, target) {
				return true
			}
		}
	}
	return false
}
