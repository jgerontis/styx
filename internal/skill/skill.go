package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var validName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Skill contains Agent Skills metadata and its directory location.
type Skill struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
	Dir           string            `yaml:"-"`
	body          string
}

// AllowedToolNames returns the frontmatter's space-separated tool names.
func (s Skill) AllowedToolNames() []string {
	return strings.Fields(s.AllowedTools)
}

// Instructions loads a skill's Markdown body only when it is activated.
func (s Skill) Instructions() (string, error) {
	if s.body != "" {
		return s.body, nil
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, "SKILL.md"))
	if err != nil {
		return "", err
	}
	_, body, err := splitSkillFile(string(data))
	if err != nil {
		return "", err
	}
	return body, nil
}

func load(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	frontmatter, _, err := splitSkillFile(string(data))
	if err != nil {
		return Skill{}, fmt.Errorf("parse %s: %w", path, err)
	}
	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return Skill{}, fmt.Errorf("parse skill metadata: %w", err)
	}
	skill.Dir = filepath.Dir(path)
	if err := skill.validate(); err != nil {
		return Skill{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return skill, nil
}

func (s Skill) validate() error {
	if len(s.Name) == 0 || len(s.Name) > 64 || !validName.MatchString(s.Name) {
		return fmt.Errorf("name must be 1-64 lowercase alphanumeric or hyphen characters")
	}
	if s.Name != filepath.Base(s.Dir) {
		return fmt.Errorf("name %q must match directory %q", s.Name, filepath.Base(s.Dir))
	}
	if len(s.Description) == 0 || len(s.Description) > 1024 {
		return fmt.Errorf("description must be 1-1024 characters")
	}
	if len(s.Compatibility) > 500 {
		return fmt.Errorf("compatibility must be at most 500 characters")
	}
	return nil
}

func splitSkillFile(content string) (string, string, error) {
	content = strings.TrimPrefix(content, "\ufeff")
	if !strings.HasPrefix(content, "---\n") {
		return "", "", fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	rest := content[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", "", fmt.Errorf("SKILL.md frontmatter must end with ---")
	}
	return rest[:end], rest[end+5:], nil
}
