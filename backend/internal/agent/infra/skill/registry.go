package skill

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type Skill struct {
	Name        string
	Description string
	Path        string
	Body        string
}

type SkillSummary struct {
	Name        string
	Description string
}

type Registry struct {
	skills map[string]Skill
	order  []string
}

func NewRegistry(roots ...string) (*Registry, error) {
	registry := &Registry{
		skills: make(map[string]Skill),
	}

	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("读取 skill 根目录失败 %s: %w", root, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillPath := filepath.Join(root, entry.Name(), "SKILL.md")
			skill, err := loadSkill(skillPath)
			if err != nil {
				return nil, err
			}
			if skill.Name == "" {
				continue
			}
			if _, exists := registry.skills[skill.Name]; exists {
				continue
			}
			registry.skills[skill.Name] = skill
			registry.order = append(registry.order, skill.Name)
		}
	}

	sort.Strings(registry.order)
	return registry, nil
}

func (r *Registry) Summaries() []Skill {
	if r == nil || len(r.order) == 0 {
		return nil
	}
	items := make([]Skill, 0, len(r.order))
	for _, name := range r.order {
		items = append(items, r.skills[name])
	}
	return items
}

func (r *Registry) Load(names []string) []Skill {
	if r == nil || len(names) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(names))
	items := make([]Skill, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		item, ok := r.skills[name]
		if !ok {
			continue
		}
		seen[name] = struct{}{}
		items = append(items, item)
	}
	return items
}

// SummariesByNames 只返回 skill 的名称和说明。
// 当子图只需要给模型注入 metadata 时，不必把正文整段加载出来。
func (r *Registry) SummariesByNames(names []string) []SkillSummary {
	if r == nil || len(names) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(names))
	items := make([]SkillSummary, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		item, ok := r.skills[name]
		if !ok {
			continue
		}
		seen[name] = struct{}{}
		items = append(items, SkillSummary{
			Name:        item.Name,
			Description: item.Description,
		})
	}
	return items
}

func loadSkill(path string) (Skill, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Skill{}, nil
		}
		return Skill{}, fmt.Errorf("读取 skill 文件失败 %s: %w", path, err)
	}

	manifest, content, err := parseSkillFile(body)
	if err != nil {
		return Skill{}, fmt.Errorf("解析 skill 文件失败 %s: %w", path, err)
	}

	referencesText, err := loadReferences(filepath.Dir(path))
	if err != nil {
		return Skill{}, err
	}
	if referencesText != "" {
		content = strings.TrimSpace(content) + "\n\n参考资料：\n" + referencesText
	}

	return Skill{
		Name:        strings.TrimSpace(manifest.Name),
		Description: strings.TrimSpace(manifest.Description),
		Path:        path,
		Body:        strings.TrimSpace(content),
	}, nil
}

func parseSkillFile(raw []byte) (Manifest, string, error) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return Manifest{}, strings.TrimSpace(text), nil
	}
	rest := strings.TrimPrefix(text, "---\n")
	index := strings.Index(rest, "\n---\n")
	if index < 0 {
		return Manifest{}, "", fmt.Errorf("无效的 front matter")
	}

	var manifest Manifest
	if err := yaml.Unmarshal([]byte(rest[:index]), &manifest); err != nil {
		return Manifest{}, "", err
	}
	content := strings.TrimSpace(rest[index+len("\n---\n"):])
	return manifest, content, nil
}

func loadReferences(skillDir string) (string, error) {
	referencesDir := filepath.Join(skillDir, "references")
	entries, err := os.ReadDir(referencesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("读取 skill 参考资料目录失败 %s: %w", referencesDir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	var builder bytes.Buffer
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(referencesDir, name))
		if err != nil {
			return "", fmt.Errorf("读取 skill 参考资料失败 %s: %w", name, err)
		}
		builder.WriteString("## ")
		builder.WriteString(name)
		builder.WriteString("\n")
		builder.WriteString(strings.TrimSpace(string(content)))
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String()), nil
}

func RenderSkillText(skills []Skill) string {
	if len(skills) == 0 {
		return "none"
	}
	var builder strings.Builder
	for _, item := range skills {
		builder.WriteString("## ")
		builder.WriteString(item.Name)
		builder.WriteString("\n")
		if item.Description != "" {
			builder.WriteString("说明：")
			builder.WriteString(strings.TrimSpace(item.Description))
			builder.WriteString("\n")
		}
		builder.WriteString(item.Body)
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String())
}

func RenderSkillSummaryText(skills []SkillSummary) string {
	if len(skills) == 0 {
		return "none"
	}
	var builder strings.Builder
	for _, item := range skills {
		builder.WriteString("## ")
		builder.WriteString(item.Name)
		builder.WriteString("\n")
		if item.Description != "" {
			builder.WriteString("说明：")
			builder.WriteString(strings.TrimSpace(item.Description))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}
