package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
)

func (g *globals) loadValidated(ctx context.Context) (root string, doc *fleetfile.Document, ds []diag.Diagnostic, sha string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, nil, "", g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".", "runtime.cwd", err.Error(), "run the command from a readable directory"),
		}})
	}
	root, err = fleetfile.FindRoot(cwd)
	if err != nil {
		return "", nil, nil, "", g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".", "fleetfile.missing",
				"no Fleetfile found in this directory or its parents",
				"run eve-fleet init <name>"),
		}})
	}
	var raw []byte
	doc, raw, err = fleetfile.Load(root)
	if err != nil {
		return "", nil, nil, "", g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error("Fleetfile", "fleetfile.parse", err.Error(), "fix YAML syntax in Fleetfile"),
		}})
	}
	ctx, cancel := context.WithTimeout(ctx, fleetfile.GitTimeout)
	defer cancel()
	sha, err = fleetfile.RevParse(ctx, root)
	if err != nil || sha == "" {
		return "", nil, nil, "", g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".", "runtime.git.pin",
				fmt.Sprintf("git HEAD must be a real commit SHA: %v", err),
				"commit the Fleetfile so revision is pinned"),
		}})
	}
	dirty, err := fleetfile.Dirty(ctx, root)
	if err != nil {
		return "", nil, nil, "", g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".", "runtime.git.required", err.Error(), "ensure git is available"),
		}})
	}
	if dirty {
		return "", nil, nil, "", g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".", "deploy.dirty",
				"work tree is dirty; build/deploy pin the git SHA, not uncommitted topology",
				"commit changes, then retry"),
		}})
	}
	ds = fleetfile.Validate(ctx, doc, root, raw)
	return root, doc, ds, sha, nil
}
