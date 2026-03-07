package skill

import "fmt"

// AccessPolicy enforces hierarchical skill access control across the layered twin architecture.
//
// Rules:
//   - An agent may only invoke a skill if it manages at least one twin whose Level is ≤
//     the skill's MinTwinLevel (i.e. equal or higher priority).
//   - OS-level agents (TwinLevelOS) may always invoke any skill.
//   - Workload-level agents may only invoke skills whose MinTwinLevel is TwinLevelWorkload.
//   - An OS-level actor may explicitly grant a workload-level agent access to a
//     restricted skill by adding a grant to the policy.
type AccessPolicy struct {
	// grants maps "agentID:skillName" → true for explicit grants from OS-level actors.
	grants map[string]bool
}

// NewAccessPolicy returns an empty access policy.
func NewAccessPolicy() *AccessPolicy {
	return &AccessPolicy{grants: make(map[string]bool)}
}

// grantKey builds the map key for an explicit grant.
func grantKey(agentID, skillName string) string {
	return agentID + ":" + skillName
}

// Grant explicitly allows agentID to invoke skillName regardless of twin level.
// Only OS-level agents should call this in production; the check is enforced by the
// caller (e.g. the server-side skill distributor).
func (p *AccessPolicy) Grant(agentID, skillName string) {
	p.grants[grantKey(agentID, skillName)] = true
}

// Revoke removes an explicit grant.
func (p *AccessPolicy) Revoke(agentID, skillName string) {
	delete(p.grants, grantKey(agentID, skillName))
}

// Check returns nil if agentID (managing twins with the given levels) is allowed to
// invoke skill. It returns a descriptive error otherwise.
//
// Decision order:
//  1. Explicit grant → allow.
//  2. Agent manages an OS-level twin → allow (OS agents are unrestricted).
//  3. Skill's MinTwinLevel == TwinLevelWorkload → allow (skill is open to all).
//  4. Otherwise → deny.
func (p *AccessPolicy) Check(agentID string, agentLevels []TwinLevel, s Skill) error {
	if p.grants[grantKey(agentID, s.Name())] {
		return nil
	}

	for _, lvl := range agentLevels {
		if lvl == TwinLevelOS {
			return nil
		}
	}

	if s.MinTwinLevel() == TwinLevelWorkload {
		return nil
	}

	return fmt.Errorf(
		"access denied: skill %q requires twin level %q; agent %q manages only workload twins",
		s.Name(), s.MinTwinLevel(), agentID,
	)
}
