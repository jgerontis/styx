package skill

import (
	"gopkg.in/yaml.v3"
)

// Builtins returns skills embedded in the Styx executable.
func Builtins() []Skill {
	return nil
}

func parseEmbedded(frontmatter, body, name string) (Skill, error) {
	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return Skill{}, err
	}
	skill.Dir = name
	skill.body = body
	if err := skill.validate(); err != nil {
		return Skill{}, err
	}
	return skill, nil
}
