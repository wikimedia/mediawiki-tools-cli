package ziki

import "testing"

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
