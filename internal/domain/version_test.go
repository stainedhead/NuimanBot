package domain

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *SkillVersion
		wantErr bool
	}{
		{
			name:  "simple version",
			input: "1.2.3",
			want:  &SkillVersion{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "version with v prefix",
			input: "v2.0.0",
			want:  &SkillVersion{Major: 2, Minor: 0, Patch: 0},
		},
		{
			name:  "version with pre-release",
			input: "1.0.0-alpha",
			want:  &SkillVersion{Major: 1, Minor: 0, Patch: 0, Pre: "alpha"},
		},
		{
			name:  "version with build metadata",
			input: "1.0.0+build.123",
			want:  &SkillVersion{Major: 1, Minor: 0, Patch: 0, Build: "build.123"},
		},
		{
			name:  "version with pre-release and build",
			input: "1.0.0-beta+exp.sha.5114f85",
			want:  &SkillVersion{Major: 1, Minor: 0, Patch: 0, Pre: "beta", Build: "exp.sha.5114f85"},
		},
		{
			name:    "invalid version",
			input:   "not.a.version",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "missing patch",
			input:   "1.2",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Major != tt.want.Major || got.Minor != tt.want.Minor || got.Patch != tt.want.Patch {
				t.Errorf("ParseVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if got.Pre != tt.want.Pre {
				t.Errorf("ParseVersion(%q) Pre = %q, want %q", tt.input, got.Pre, tt.want.Pre)
			}
			if got.Build != tt.want.Build {
				t.Errorf("ParseVersion(%q) Build = %q, want %q", tt.input, got.Build, tt.want.Build)
			}
		})
	}
}

func TestSkillVersion_String(t *testing.T) {
	tests := []struct {
		name string
		v    *SkillVersion
		want string
	}{
		{
			name: "simple version",
			v:    &SkillVersion{Major: 1, Minor: 2, Patch: 3},
			want: "1.2.3",
		},
		{
			name: "with pre-release",
			v:    &SkillVersion{Major: 1, Minor: 0, Patch: 0, Pre: "alpha"},
			want: "1.0.0-alpha",
		},
		{
			name: "with build",
			v:    &SkillVersion{Major: 2, Minor: 1, Patch: 0, Build: "build.1"},
			want: "2.1.0+build.1",
		},
		{
			name: "with pre and build",
			v:    &SkillVersion{Major: 1, Minor: 0, Patch: 0, Pre: "rc.1", Build: "20231010"},
			want: "1.0.0-rc.1+20231010",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSkillVersion_Compare(t *testing.T) {
	tests := []struct {
		name string
		v1   *SkillVersion
		v2   *SkillVersion
		want int
	}{
		{
			name: "equal versions",
			v1:   &SkillVersion{Major: 1, Minor: 2, Patch: 3},
			v2:   &SkillVersion{Major: 1, Minor: 2, Patch: 3},
			want: 0,
		},
		{
			name: "v1 greater major",
			v1:   &SkillVersion{Major: 2, Minor: 0, Patch: 0},
			v2:   &SkillVersion{Major: 1, Minor: 9, Patch: 9},
			want: 1,
		},
		{
			name: "v1 lesser major",
			v1:   &SkillVersion{Major: 1, Minor: 0, Patch: 0},
			v2:   &SkillVersion{Major: 2, Minor: 0, Patch: 0},
			want: -1,
		},
		{
			name: "v1 greater minor",
			v1:   &SkillVersion{Major: 1, Minor: 3, Patch: 0},
			v2:   &SkillVersion{Major: 1, Minor: 2, Patch: 9},
			want: 1,
		},
		{
			name: "v1 lesser minor",
			v1:   &SkillVersion{Major: 1, Minor: 1, Patch: 0},
			v2:   &SkillVersion{Major: 1, Minor: 2, Patch: 0},
			want: -1,
		},
		{
			name: "v1 greater patch",
			v1:   &SkillVersion{Major: 1, Minor: 2, Patch: 4},
			v2:   &SkillVersion{Major: 1, Minor: 2, Patch: 3},
			want: 1,
		},
		{
			name: "v1 lesser patch",
			v1:   &SkillVersion{Major: 1, Minor: 2, Patch: 2},
			v2:   &SkillVersion{Major: 1, Minor: 2, Patch: 3},
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v1.Compare(tt.v2); got != tt.want {
				t.Errorf("Compare() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseConstraint(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantOperator string
		wantErr      bool
	}{
		{
			name:         "exact version",
			input:        "1.2.3",
			wantOperator: "=",
		},
		{
			name:         "caret constraint",
			input:        "^1.2.3",
			wantOperator: "^",
		},
		{
			name:         "tilde constraint",
			input:        "~2.0.0",
			wantOperator: "~",
		},
		{
			name:    "unsupported constraint",
			input:   ">=1.0.0",
			wantErr: true,
		},
		{
			name:    "invalid version in constraint",
			input:   "^invalid",
			wantErr: true,
		},
		{
			name:    "tilde with invalid version",
			input:   "~invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConstraint(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConstraint(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Operator != tt.wantOperator {
				t.Errorf("ParseConstraint(%q) operator = %q, want %q", tt.input, got.Operator, tt.wantOperator)
			}
		})
	}
}

func TestVersionConstraint_Satisfies(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		version    string
		want       bool
	}{
		{
			name:       "exact match satisfied",
			constraint: "1.2.3",
			version:    "1.2.3",
			want:       true,
		},
		{
			name:       "exact match not satisfied",
			constraint: "1.2.3",
			version:    "1.2.4",
			want:       false,
		},
		{
			name:       "caret same major satisfied",
			constraint: "^1.0.0",
			version:    "1.5.0",
			want:       true,
		},
		{
			name:       "caret different major not satisfied",
			constraint: "^1.0.0",
			version:    "2.0.0",
			want:       false,
		},
		{
			name:       "caret older version not satisfied",
			constraint: "^1.2.0",
			version:    "1.1.0",
			want:       false,
		},
		{
			name:       "tilde same major minor satisfied",
			constraint: "~1.2.0",
			version:    "1.2.5",
			want:       true,
		},
		{
			name:       "tilde different minor not satisfied",
			constraint: "~1.2.0",
			version:    "1.3.0",
			want:       false,
		},
		{
			name:       "tilde different major not satisfied",
			constraint: "~1.2.0",
			version:    "2.2.0",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ParseConstraint(tt.constraint)
			if err != nil {
				t.Fatalf("ParseConstraint(%q) error = %v", tt.constraint, err)
			}

			v, err := ParseVersion(tt.version)
			if err != nil {
				t.Fatalf("ParseVersion(%q) error = %v", tt.version, err)
			}

			if got := c.Satisfies(v); got != tt.want {
				t.Errorf("Satisfies(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestVersionConstraint_Satisfies_UnknownOperator(t *testing.T) {
	c := &VersionConstraint{
		Operator: "!",
		Version:  &SkillVersion{Major: 1, Minor: 0, Patch: 0},
	}
	v := &SkillVersion{Major: 1, Minor: 0, Patch: 0}
	if c.Satisfies(v) {
		t.Error("unknown operator should return false")
	}
}
