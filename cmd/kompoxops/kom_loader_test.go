package main

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestGetKOMPaths(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		flagVals []string
		want     []string
	}{
		{
			name:     "no paths",
			envValue: "",
			flagVals: nil,
			want:     nil,
		},
		{
			name:     "env only",
			envValue: "path1,path2",
			flagVals: nil,
			want:     []string{"path1", "path2"},
		},
		{
			name:     "env with spaces",
			envValue: " path1 , path2 ",
			flagVals: nil,
			want:     []string{"path1", "path2"},
		},
		{
			name:     "flag overrides env",
			envValue: "env1,env2",
			flagVals: []string{"flag1", "flag2"},
			want:     []string{"flag1", "flag2"},
		},
		{
			name:     "flag only",
			envValue: "",
			flagVals: []string{"flag1"},
			want:     []string{"flag1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv("KOMPOX_KOM_PATH", tt.envValue)
				defer os.Unsetenv("KOMPOX_KOM_PATH")
			} else {
				os.Unsetenv("KOMPOX_KOM_PATH")
			}

			// Create command with flags
			cmd := &cobra.Command{}
			cmd.PersistentFlags().StringArray("kom-path", nil, "")

			// Parse flags to simulate command-line usage
			if tt.flagVals != nil {
				args := []string{}
				for _, v := range tt.flagVals {
					args = append(args, "--kom-path="+v)
				}
				cmd.SetArgs(args)
				cmd.ParseFlags(args)
			}

			got := getKOMPaths(cmd)

			// Compare results
			if len(got) != len(tt.want) {
				t.Errorf("getKOMPaths() length = %v, want %v (got: %v)", len(got), len(tt.want), got)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("getKOMPaths()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetKOMAppPath(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		flagVal  string
		want     string
	}{
		{
			name:     "default",
			envValue: "",
			flagVal:  "",
			want:     "./kompoxapp.yml",
		},
		{
			name:     "env only",
			envValue: "custom.yml",
			flagVal:  "",
			want:     "custom.yml",
		},
		{
			name:     "flag overrides env",
			envValue: "env.yml",
			flagVal:  "flag.yml",
			want:     "flag.yml",
		},
		{
			name:     "flag only",
			envValue: "",
			flagVal:  "flag.yml",
			want:     "flag.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv("KOMPOX_KOM_APP", tt.envValue)
				defer os.Unsetenv("KOMPOX_KOM_APP")
			} else {
				os.Unsetenv("KOMPOX_KOM_APP")
			}

			// Create command with flag
			cmd := &cobra.Command{}
			cmd.PersistentFlags().String("kom-app", "./kompoxapp.yml", "")

			// Parse flags if needed
			if tt.flagVal != "" {
				args := []string{"--kom-app=" + tt.flagVal}
				cmd.SetArgs(args)
				cmd.ParseFlags(args)
			}

			got := getKOMAppPath(cmd)

			if got != tt.want {
				t.Errorf("getKOMAppPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
