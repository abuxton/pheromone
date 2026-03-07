package skill

import (
	"fmt"
	"sync"
)

// SkillRegistry holds all skills available for deployment and provides distribution
// to agents based on their twin-level access policy.
//
// In the Pheromone model the server owns the canonical registry. Agents receive a
// filtered subset of skills (their "skill bundle") at registration time — only the
// skills they are permitted to invoke based on the access policy.
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]Skill
	policy *AccessPolicy
}

// NewSkillRegistry creates an empty registry with the provided access policy.
func NewSkillRegistry(policy *AccessPolicy) *SkillRegistry {
	return &SkillRegistry{
		skills: make(map[string]Skill),
		policy: policy,
	}
}

// Register adds a skill to the registry. It returns an error if a skill with the
// same name is already registered.
func (r *SkillRegistry) Register(s Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[s.Name()]; exists {
		return fmt.Errorf("skill %q is already registered", s.Name())
	}
	r.skills[s.Name()] = s
	return nil
}

// Deregister removes a skill from the registry.
func (r *SkillRegistry) Deregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[name]; !exists {
		return fmt.Errorf("skill %q not found", name)
	}
	delete(r.skills, name)
	return nil
}

// Get returns the skill registered under name, or an error if not found.
func (r *SkillRegistry) Get(name string) (Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.skills[name]
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	return s, nil
}

// List returns all registered skills in an unspecified order.
func (r *SkillRegistry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	return out
}

// BundleFor returns the subset of skills that agentID is permitted to invoke given
// its managed twin levels. This is the "skill bundle" distributed to the agent at
// registration time.
func (r *SkillRegistry) BundleFor(agentID string, twinLevels []TwinLevel) []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var bundle []Skill
	for _, s := range r.skills {
		if err := r.policy.Check(agentID, twinLevels, s); err == nil {
			bundle = append(bundle, s)
		}
	}
	return bundle
}
