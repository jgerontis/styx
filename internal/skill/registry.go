package skill

import (
	"os"
	"path/filepath"
	"sort"
)

// Registry discovers skills without loading their instruction bodies.
type Registry struct {
	builtins []Skill
	roots    []string
	skills   map[string]Skill
}

// NewRegistry creates a skill registry for the provided skill roots.
func NewRegistry(roots ...string) *Registry {
	return &Registry{roots: roots, skills: make(map[string]Skill)}
}

// NewRegistryWithBuiltins creates a registry whose roots override built-in skills.
func NewRegistryWithBuiltins(builtins []Skill, roots ...string) *Registry {
	return &Registry{builtins: builtins, roots: roots, skills: make(map[string]Skill)}
}

// Discover loads metadata from immediate child directories containing SKILL.md.
func (r *Registry) Discover() error {
	r.skills = make(map[string]Skill)
	for _, skill := range r.builtins {
		r.skills[skill.Name] = skill
	}
	for _, root := range r.roots {
		entries, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skill, err := load(filepath.Join(root, entry.Name(), "SKILL.md"))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			r.skills[skill.Name] = skill
		}
	}
	return nil
}

// All returns discovered skills ordered by name.
func (r *Registry) All() []Skill {
	skills := make([]Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		skills = append(skills, skill)
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills
}

// Get returns a discovered skill by name.
func (r *Registry) Get(name string) (Skill, bool) {
	skill, ok := r.skills[name]
	return skill, ok
}
