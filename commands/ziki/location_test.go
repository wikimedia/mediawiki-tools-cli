package ziki

import (
	"strings"
	"testing"
)

func TestLocationNameFromString(t *testing.T) {
	type args struct {
		inputName string
	}
	tests := []struct {
		name    string
		args    args
		want    LocationName
		wantErr bool
	}{
		{
			name: "Full name matches exactly",
			args: args{
				inputName: string(Gerrit),
			},
			want:    Gerrit,
			wantErr: false,
		},
		{
			name: "Shorthand also matches",
			args: args{
				inputName: "Ema",
			},
			want:    Email,
			wantErr: false,
		},
		{
			name: "Single letters match when unique",
			args: args{
				inputName: "M",
			},
			want:    Meeting,
			wantErr: false,
		},
		{
			name: "Single letters with multiple matches is an error",
			args: args{
				inputName: "G",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "No match is error",
			args: args{
				inputName: "XXXXXXXX",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LocationNameFromString(tt.args.inputName)
			if (err != nil) != tt.wantErr {
				t.Errorf("LocationNameFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("LocationNameFromString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDestinationAliases(t *testing.T) {
	seen := map[string]LocationName{}
	for destination, aliases := range locationAliases {
		for _, alias := range aliases {
			normalized := normalizeDestination(alias)
			if previous, exists := seen[normalized]; exists && previous != destination {
				t.Fatalf("alias %q maps to both %s and %s", alias, previous, destination)
			}
			seen[normalized] = destination
			for _, input := range []string{alias, "  " + strings.ToUpper(alias) + "  ", strings.ReplaceAll(alias, " ", "-"), strings.ReplaceAll(alias, " ", "_")} {
				got, err := LocationNameFromString(input)
				if err != nil || got != destination {
					t.Errorf("%q = %s, %v; want %s", input, got, err, destination)
				}
			}
		}
	}
	for _, input := range []string{"g", "p", ""} {
		if _, err := LocationNameFromString(input); err == nil {
			t.Errorf("%q should remain ambiguous or invalid", input)
		}
	}
}

func TestTravelWithAliases(t *testing.T) {
	g, out := testGame("")
	g.Player.CurrentLocation = CommandLine
	for _, tc := range []struct {
		input string
		want  LocationName
	}{{"phabricator", Phab}, {"go cli", CommandLine}, {"tg", Telegram}, {"go prod", Production}, {"slack", Production}} {
		ProcessCommands(g, tc.input)
		if g.Player.CurrentLocation != tc.want {
			t.Errorf("%q moved to %s, want %s", tc.input, g.Player.CurrentLocation, tc.want)
		}
	}
	if !strings.Contains(out.String(), "ACCESS DENIED") {
		t.Fatal("Slack alias bypassed the locked door")
	}
}
