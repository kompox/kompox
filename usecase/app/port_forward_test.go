package app

import "testing"

func TestParsePortSpec(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantLocal  int
		wantRemote int
		wantErr    bool
	}{
		{name: "local:remote", in: "8080:80", wantLocal: 8080, wantRemote: 80},
		{name: "same port", in: "8080", wantLocal: 8080, wantRemote: 8080},
		{name: "auto local", in: ":80", wantLocal: 0, wantRemote: 80},
		{name: "spaces", in: "  :3000 ", wantLocal: 0, wantRemote: 3000},
		{name: "local zero allowed", in: "0:80", wantLocal: 0, wantRemote: 80},
		{name: "invalid empty", in: "", wantErr: true},
		{name: "invalid colon only", in: ":", wantErr: true},
		{name: "invalid remote missing", in: "80:", wantErr: true},
		{name: "invalid non-numeric", in: "abc", wantErr: true},
		{name: "invalid remote non-numeric", in: "8080:abc", wantErr: true},
		{name: "invalid local non-numeric", in: "abc:80", wantErr: true},
		{name: "invalid remote zero", in: "8080:0", wantErr: true},
		{name: "invalid local negative", in: "-1:80", wantErr: true},
		{name: "invalid single zero", in: "0", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp, rp, err := parsePortSpec(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePortSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if lp != tt.wantLocal || rp != tt.wantRemote {
				t.Fatalf("parsePortSpec() = (%d,%d), want (%d,%d)", lp, rp, tt.wantLocal, tt.wantRemote)
			}
		})
	}
}

func TestSplitAddresses(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "default", in: "", want: []string{"localhost"}},
		{name: "single", in: "0.0.0.0", want: []string{"0.0.0.0"}},
		{name: "multi", in: "0.0.0.0,localhost", want: []string{"0.0.0.0", "localhost"}},
		{name: "trim", in: " 0.0.0.0 , localhost ", want: []string{"0.0.0.0", "localhost"}},
		{name: "empty parts", in: ",,localhost,,", want: []string{"localhost"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAddresses(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("splitAddresses() len=%d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("splitAddresses()[%d]=%q, want %q (%v)", i, got[i], tt.want[i], got)
				}
			}
		})
	}
}
