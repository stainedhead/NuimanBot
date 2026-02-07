package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SkillVersion represents a semantic version
type SkillVersion struct {
	Major int
	Minor int
	Patch int
	Pre   string
	Build string
}

// ParseVersion parses a semver string
func ParseVersion(v string) (*SkillVersion, error) {
	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")

	// Basic semver pattern
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9.-]+))?(?:\+([a-zA-Z0-9.-]+))?$`)
	matches := re.FindStringSubmatch(v)

	if matches == nil {
		return nil, fmt.Errorf("invalid version format: %s", v)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return &SkillVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
		Pre:   matches[4],
		Build: matches[5],
	}, nil
}

// String returns the version as a string
func (v *SkillVersion) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare compares two versions (-1 if less, 0 if equal, 1 if greater)
func (v *SkillVersion) Compare(other *SkillVersion) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	return 0
}

// VersionConstraint represents a version constraint (e.g., ^1.0.0, ~2.1.0)
type VersionConstraint struct {
	Operator string
	Version  *SkillVersion
}

// ParseConstraint parses a version constraint
func ParseConstraint(c string) (*VersionConstraint, error) {
	c = strings.TrimSpace(c)

	// Exact version
	if !strings.ContainsAny(c, "^~>=<") {
		v, err := ParseVersion(c)
		if err != nil {
			return nil, err
		}
		return &VersionConstraint{Operator: "=", Version: v}, nil
	}

	// Caret (^1.0.0)
	if strings.HasPrefix(c, "^") {
		v, err := ParseVersion(c[1:])
		if err != nil {
			return nil, err
		}
		return &VersionConstraint{Operator: "^", Version: v}, nil
	}

	// Tilde (~1.0.0)
	if strings.HasPrefix(c, "~") {
		v, err := ParseVersion(c[1:])
		if err != nil {
			return nil, err
		}
		return &VersionConstraint{Operator: "~", Version: v}, nil
	}

	return nil, fmt.Errorf("unsupported constraint format: %s", c)
}

// Satisfies checks if a version satisfies the constraint
func (c *VersionConstraint) Satisfies(v *SkillVersion) bool {
	switch c.Operator {
	case "=":
		return v.Compare(c.Version) == 0
	case "^": // Compatible with (same major version)
		return v.Major == c.Version.Major && v.Compare(c.Version) >= 0
	case "~": // Approximately (same major.minor)
		return v.Major == c.Version.Major &&
			v.Minor == c.Version.Minor &&
			v.Compare(c.Version) >= 0
	default:
		return false
	}
}
