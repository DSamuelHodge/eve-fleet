package cli

import (
	"context"
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
			return g.runInit(cmd.Context(), args[0])
		},
	}
	return cmd
}

func (g *globals) runInit(ctx context.Context, name string) error {
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
	if err := os.Mkdir(root, 0o755); err != nil {
		if os.IsExist(err) {
			return g.report(diag.Report{
				OK: false,
				Diagnostics: []diag.Diagnostic{
					diag.Error(name, "init.dir.exists",
						fmt.Sprintf("directory %s already exists", name),
						"choose a new name or remove the directory"),
				},
			})
		}
		return err
	}
	cleanup := func() { _ = os.RemoveAll(root) }

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
			cleanup()
			return err
		}
		keep := filepath.Join(d, ".gitkeep")
		if err := os.WriteFile(keep, []byte{}, 0o644); err != nil {
			cleanup()
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(root, fleetfile.FileName), fleetfile.ScaffoldYAML(name), 0o644); err != nil {
		cleanup()
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, fleetfile.GitTimeout)
	defer cancel()
	if err := gitInitCommit(ctx, root, name); err != nil {
		cleanup()
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error(".", "runtime.git.required",
					fmt.Sprintf("git init/commit failed: %v", err),
					"install git and ensure the directory is writable"),
			},
		})
	}
	sha, err := fleetfile.RevParse(ctx, root)
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

func gitInitCommit(ctx context.Context, root, name string) error {
	steps := []struct {
		name string
		args []string
	}{
		{name: "init", args: []string{"init", "--template="}},
		{name: "add", args: []string{"add", "-A"}},
		{name: "commit", args: []string{"-c", "user.name=eve-fleet", "-c", "user.email=eve-fleet@local", "commit", "-m", "Initial fleet " + name}},
	}
	for _, step := range steps {
		cmd := exec.CommandContext(ctx, "git", step.args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=eve-fleet",
			"GIT_AUTHOR_EMAIL=eve-fleet@local",
			"GIT_COMMITTER_NAME=eve-fleet",
			"GIT_COMMITTER_EMAIL=eve-fleet@local",
			"GIT_TERMINAL_PROMPT=0",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w (%s)", step.name, err, out)
		}
	}
	return nil
}
