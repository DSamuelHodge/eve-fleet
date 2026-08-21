package cli

import (
	"context"
	"os"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
)

func (g *globals) loadValidated(ctx context.Context) (root string, doc *fleetfile.Document, ds []diag.Diagnostic, sha string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, nil, "", err
	}
	root, err = fleetfile.FindRoot(cwd)
	if err != nil {
		return "", nil, nil, "", g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".", "fleetfile.missing",
				"no Fleetfile found in this directory or its parents",
				"run eve-fleet init <name>"),
		}})
	}
	doc, _, err = fleetfile.Load(root)
	if err != nil {
		return "", nil, nil, "", g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error("Fleetfile", "fleetfile.parse", err.Error(), "fix YAML syntax in Fleetfile"),
		}})
	}
	ctx, cancel := context.WithTimeout(ctx, fleetfile.GitTimeout)
	defer cancel()
	ds = fleetfile.Validate(ctx, doc, root)
	sha, _ = fleetfile.RevParse(ctx, root)
	return root, doc, ds, sha, nil
}
