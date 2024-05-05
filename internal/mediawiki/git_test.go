package mediawiki

import "testing"

func TestCloneOpts_AreThereThingsToClone(t *testing.T) {
	tests := []struct {
		name string
		cp   CloneOpts
		want bool
	}{
		{
			name: "GetMediaWiki is true",
			cp: CloneOpts{
				GetMediaWiki: true,
			},
			want: true,
		},
		{
			name: "GetVector is true",
			cp: CloneOpts{
				GetVector: true,
			},
			want: true,
		},
		{
			name: "GetGerritSkins is not empty",
			cp: CloneOpts{
				GetGerritSkins: []string{"skin1", "skin2"},
			},
			want: true,
		},
		{
			name: "GetGerritExtensions is not empty",
			cp: CloneOpts{
				GetGerritExtensions: []string{"extension1", "extension2"},
			},
			want: true,
		},
		{
			name: "All options are false or empty",
			cp:   CloneOpts{},
			want: false,
		},
		{
			name: "extensions set but empty",
			cp: CloneOpts{
				GetGerritExtensions: []string{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cp.AreThereThingsToClone()
			if got != tt.want {
				t.Errorf("CloneOpts.AreThereThingsToClone() = %v, want %v", got, tt.want)
			}
		})
	}
}
