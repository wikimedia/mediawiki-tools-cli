package mediawiki

import "testing"

func TestProjectToLocalDir(t *testing.T) {
	tests := []struct {
		name      string
		project   string
		want      string
		wantError bool
	}{
		{
			name:    "core project",
			project: "mediawiki/core",
			want:    "",
		},
		{
			name:    "extension project",
			project: "mediawiki/extensions/Examples",
			want:    "extensions/Examples",
		},
		{
			name:    "skin project",
			project: "mediawiki/skins/Vector",
			want:    "skins/Vector",
		},
		{
			name:      "unsupported project",
			project:   "operations/puppet",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProjectToLocalDir(tt.project)
			if tt.wantError {
				if err == nil {
					t.Errorf("ProjectToLocalDir(%q) expected error, got nil", tt.project)
				}
				return
			}
			if err != nil {
				t.Errorf("ProjectToLocalDir(%q) unexpected error: %v", tt.project, err)
				return
			}
			if got != tt.want {
				t.Errorf("ProjectToLocalDir(%q) = %q, want %q", tt.project, got, tt.want)
			}
		})
	}
}
