package skill

import (
	"fmt"

	"nuimanbot/internal/domain"
)

// VersionResolver resolves skill version constraints
type VersionResolver struct{}

// NewVersionResolver creates a new version resolver
func NewVersionResolver() *VersionResolver {
	return &VersionResolver{}
}

// ResolveVersion finds the best version matching a constraint
func (r *VersionResolver) ResolveVersion(available []*domain.SkillVersion, constraint string) (*domain.SkillVersion, error) {
	c, err := domain.ParseConstraint(constraint)
	if err != nil {
		return nil, fmt.Errorf("invalid constraint: %w", err)
	}

	var best *domain.SkillVersion
	for _, v := range available {
		if c.Satisfies(v) {
			if best == nil || v.Compare(best) > 0 {
				best = v
			}
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no version satisfies constraint: %s", constraint)
	}

	return best, nil
}

// CheckCompatibility checks if two versions are compatible
func (r *VersionResolver) CheckCompatibility(v1, v2 *domain.SkillVersion) bool {
	// Same major version = compatible
	return v1.Major == v2.Major
}
