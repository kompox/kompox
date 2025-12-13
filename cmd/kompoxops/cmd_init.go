package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kompox/kompox/config/kompoxenv"
	"github.com/spf13/cobra"
)

func newCmdInit() *cobra.Command {
	var forceFlag bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Kompox CLI Env",
		Long: `Initialize Kompox CLI Env by creating .kompox/ directory structure and config.yml.

The init command creates:
  - .kompox/ directory
  - .kompox/config.yml with default configuration
  - .kompox/kom/ directory (default KOM file location)

If -C is specified and the directory does not exist, it will be created
recursively (including parent directories). This is init-specific behavior;
other commands will error if the -C directory does not exist.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, args, forceFlag)
		},
	}

	cmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Overwrite existing .kompox/config.yml")
	return cmd
}

func runInit(cmd *cobra.Command, args []string, forceFlag bool) error {
	// Handle -C flag manually for init command (PersistentPreRunE skips init)
	// Try to get from both local and persistent flags
	var dir string
	if cmd.Flags().Changed("chdir") {
		dir, _ = cmd.Flags().GetString("chdir")
	} else if cmd.Parent() != nil && cmd.Parent().PersistentFlags().Changed("chdir") {
		dir, _ = cmd.Parent().PersistentFlags().GetString("chdir")
	}

	if dir != "" {
		// Create the directory if it doesn't exist (init-specific behavior)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %q: %w", dir, err)
		}
		if err := os.Chdir(dir); err != nil {
			return fmt.Errorf("changing directory to %q: %w", dir, err)
		}
	}

	// Get working directory (after -C flag processing)
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Define paths
	kompoxDir := filepath.Join(workDir, kompoxenv.KompoxDirName)
	configPath := filepath.Join(kompoxDir, kompoxenv.ConfigFileName)
	komDir := filepath.Join(kompoxDir, "kom")
	logsDir := filepath.Join(kompoxDir, "logs")
	gitignorePath := filepath.Join(kompoxDir, ".gitignore")

	// Check if config.yml already exists
	if !forceFlag {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("%s already exists (use -f to overwrite)", configPath)
		}
	}

	// Create .kompox/ directory
	if err := os.MkdirAll(kompoxDir, 0755); err != nil {
		return fmt.Errorf("creating %s directory: %w", kompoxDir, err)
	}

	// Create .kompox/kom/ directory
	if err := os.MkdirAll(komDir, 0755); err != nil {
		return fmt.Errorf("creating %s directory: %w", komDir, err)
	}

	// Create .kompox/logs/ directory
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("creating %s directory: %w", logsDir, err)
	}

	// Generate default config.yml content
	data, err := kompoxenv.InitialConfigYAML()
	if err != nil {
		return fmt.Errorf("generating default config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", configPath, err)
	}

	// Create .kompox/.gitignore if it doesn't exist
	gitignoreCreated := false
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		gitignoreContent := "/logs\n"
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", gitignorePath, err)
		}
		gitignoreCreated = true
	}

	fmt.Printf("Initialized Kompox CLI Env in %s\n", kompoxDir)
	fmt.Printf("Created:\n")
	fmt.Printf("  - %s\n", configPath)
	fmt.Printf("  - %s/\n", komDir)
	fmt.Printf("  - %s/\n", logsDir)
	if gitignoreCreated {
		fmt.Printf("  - %s\n", gitignorePath)
	}

	return nil
}
