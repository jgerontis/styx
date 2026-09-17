package skill

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

//go:embed builtins/ast-grep/SKILL.md
var astGrepSkill string

// Builtins returns skills embedded in the Styx executable.
func Builtins() []Skill {
	frontmatter, body, err := splitSkillFile(astGrepSkill)
	if err != nil {
		panic(err)
	}
	skill, err := parseEmbedded(frontmatter, body, "ast-grep")
	if err != nil {
		panic(err)
	}
	return []Skill{skill}
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
