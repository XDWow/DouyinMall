package skill

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	adkskill "github.com/cloudwego/eino/adk/middlewares/skill"
	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Context     string `yaml:"context"`
	Agent       string `yaml:"agent"`
	Model       string `yaml:"model"`
}

type Skill struct {
	Name        string
	Description string
	Context     string
	Agent       string
	Model       string
	Path        string
	Body        string
}

// Registry is a local SKILL.md index.
// It is intentionally small: load skills once, then build a filtered ADK backend per subgraph.
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
			return nil, fmt.Errorf("read skill root %s: %w", root, err)
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

func loadSkill(path string) (Skill, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Skill{}, nil
		}
		return Skill{}, fmt.Errorf("read skill file %s: %w", path, err)
	}

	manifest, content, err := parseSkillFile(body)
	if err != nil {
		return Skill{}, fmt.Errorf("parse skill file %s: %w", path, err)
	}

	referencesText, err := loadReferences(filepath.Dir(path))
	if err != nil {
		return Skill{}, err
	}
	if referencesText != "" {
		content = strings.TrimSpace(content) + "\n\nReferences:\n" + referencesText
	}

	return Skill{
		Name:        strings.TrimSpace(manifest.Name),
		Description: strings.TrimSpace(manifest.Description),
		Context:     strings.TrimSpace(manifest.Context),
		Agent:       strings.TrimSpace(manifest.Agent),
		Model:       strings.TrimSpace(manifest.Model),
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
		return Manifest{}, "", fmt.Errorf("invalid front matter")
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
		return "", fmt.Errorf("read skill references dir %s: %w", referencesDir, err)
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
			return "", fmt.Errorf("read skill reference %s: %w", name, err)
		}
		builder.WriteString("## ")
		builder.WriteString(name)
		builder.WriteString("\n")
		builder.WriteString(strings.TrimSpace(string(content)))
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String()), nil
}

// ADKBackend returns a backend scoped to the current subgraph whitelist.
// The agent sees only these skills.
func (r *Registry) ADKBackend(names []string) adkskill.Backend {
	if r == nil || len(names) == 0 {
		return nil
	}

	items := r.Load(names)
	if len(items) == 0 {
		return nil
	}

	allowed := make(map[string]Skill, len(items))
	order := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		allowed[item.Name] = item
		order = append(order, item.Name)
	}
	if len(order) == 0 {
		return nil
	}

	return &adkBackend{
		skills: allowed,
		order:  order,
	}
}

type adkBackend struct {
	skills map[string]Skill
	order  []string
}

func (b *adkBackend) List(context.Context) ([]adkskill.FrontMatter, error) {
	if b == nil || len(b.order) == 0 {
		return nil, nil
	}

	out := make([]adkskill.FrontMatter, 0, len(b.order))
	for _, name := range b.order {
		item, ok := b.skills[name]
		if !ok {
			continue
		}
		out = append(out, adkskill.FrontMatter{
			Name:        item.Name,
			Description: item.Description,
			Context:     adkskill.ContextMode(item.Context),
			Agent:       item.Agent,
			Model:       item.Model,
		})
	}
	return out, nil
}

func (b *adkBackend) Get(_ context.Context, name string) (adkskill.Skill, error) {
	if b == nil {
		return adkskill.Skill{}, fmt.Errorf("skill backend is nil")
	}

	item, ok := b.skills[strings.TrimSpace(name)]
	if !ok {
		return adkskill.Skill{}, fmt.Errorf("skill %q not found", name)
	}
	return adkskill.Skill{
		FrontMatter: adkskill.FrontMatter{
			Name:        item.Name,
			Description: item.Description,
			Context:     adkskill.ContextMode(item.Context),
			Agent:       item.Agent,
			Model:       item.Model,
		},
		Content:       item.Body,
		BaseDirectory: filepath.Dir(item.Path),
	}, nil
}
