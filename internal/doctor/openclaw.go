package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

type OpenClawChecker struct {
	HomeDir string
}

func NewOpenClawChecker(homeDir string) OpenClawChecker {
	return OpenClawChecker{HomeDir: homeDir}
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
	if len(missing) > 0 {
		report.Items = append(report.Items, Item{Level: LevelFail, Message: "imgen skill is installed but missing guidance markers: " + strings.Join(missing, ", ")})
		return
	}
	report.Items = append(report.Items, Item{Level: LevelOK, Message: "imgen skill installed for OpenClaw"})
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
