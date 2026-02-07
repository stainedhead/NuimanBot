package skill

import (
	"fmt"

	"nuimanbot/internal/domain"
)

// VersionManager manages skill versions
type VersionManager struct {
	versions map[string][]*domain.SkillVersion
}

// NewVersionManager creates a new version manager
func NewVersionManager() *VersionManager {
	return &VersionManager{
		versions: make(map[string][]*domain.SkillVersion),
	}
}

// RegisterVersion registers a skill version
func (m *VersionManager) RegisterVersion(skillName string, version *domain.SkillVersion) {
	m.versions[skillName] = append(m.versions[skillName], version)
}

// GetLatest returns the latest version of a skill
func (m *VersionManager) GetLatest(skillName string) (*domain.SkillVersion, error) {
	versions := m.versions[skillName]
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions available for skill: %s", skillName)
	}

	latest := versions[0]
	for _, v := range versions[1:] {
		if v.Compare(latest) > 0 {
			latest = v
		}
	}

	return latest, nil
}

// GetVersions returns all versions for a skill
func (m *VersionManager) GetVersions(skillName string) []*domain.SkillVersion {
	return m.versions[skillName]
}
