package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
	"github.com/spf13/cobra"
)

func newInitCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init <name>",
		Short:   "Scaffold a fleet repository, Fleetfile, and git",
		GroupID: groupFleet,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return g.report(diag.Report{
					OK: false,
					Diagnostics: []diag.Diagnostic{
						diag.Error(".", "init.name.required",
							"init requires a fleet name",
							"run eve-fleet init <name>"),
					},
				})
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runInit(args[0])
		},
	}
	return cmd
}

func (g *globals) runInit(name string) error {
	if !fleetfile.ValidDNSLabel(name) {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error("metadata.name", "metadata.name",
					"fleet name must be a DNS-label (lowercase letters, digits, hyphens)",
					"use a name like revenue-ops"),
			},
		})
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root := filepath.Join(cwd, name)
	if _, err := os.Stat(root); err == nil {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error(name, "init.dir.exists",
					fmt.Sprintf("directory %s already exists", name),
					"choose a new name or remove the directory"),
			},
		})
	}
	dirs := []string{
		filepath.Join(root, "agents"),
		filepath.Join(root, "shared", "skills"),
		filepath.Join(root, "shared", "tools"),
		filepath.Join(root, "shared", "connections"),
		filepath.Join(root, "shared", "lib"),
		filepath.Join(root, "evals"),
		filepath.Join(root, ".eve-fleet"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
		keep := filepath.Join(d, ".gitkeep")
		if err := os.WriteFile(keep, []byte{}, 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(root, fleetfile.FileName), fleetfile.ScaffoldYAML(name), 0o644); err != nil {
		return err
	}
	if err := gitInitCommit(root, name); err != nil {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error(".", "runtime.git.required",
					fmt.Sprintf("git init/commit failed: %v", err),
					"install git and ensure the directory is writable"),
			},
		})
	}
	sha, err := fleetfile.RevParse(root)
	if err != nil {
		return err
	}
	return g.report(diag.Report{
		OK:      true,
		Name:    name,
		Path:    root,
		GitSHA:  sha,
		PlainOK: fmt.Sprintf("Initialized fleet %s (git SHA %s)", name, sha),
	})
}

func gitInitCommit(root, name string) error {
	steps := [][]string{
		{"init"},
		{"add", "-A"},
		{"-c", "user.name=eve-fleet", "-c", "user.email=eve-fleet@local", "commit", "-m", "Initial fleet " + name},
	}
	for _, args := range steps {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=eve-fleet",
			"GIT_AUTHOR_EMAIL=eve-fleet@local",
			"GIT_COMMITTER_NAME=eve-fleet",
			"GIT_COMMITTER_EMAIL=eve-fleet@local",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w (%s)", args[0], err, out)
		}
	}
	return nil
}
