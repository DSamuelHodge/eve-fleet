package cli

import (
	"context"
	"os"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
	"github.com/spf13/cobra"
)

func newDoctorCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		Short:   "Validate topology, paths, ownership, and git pin",
		GroupID: groupInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runDoctor(cmd.Context())
		},
	}
}

func (g *globals) runDoctor(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := fleetfile.FindRoot(cwd)
	if err != nil {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error(".", "fleetfile.missing",
					"no Fleetfile found in this directory or its parents",
					"run eve-fleet init <name>"),
			},
		})
	}
	doc, _, err := fleetfile.Load(root)
	if err != nil {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error("Fleetfile", "fleetfile.parse",
					err.Error(),
					"fix YAML syntax in Fleetfile"),
			},
		})
	}
	ctx, cancel := context.WithTimeout(ctx, fleetfile.GitTimeout)
	defer cancel()
	ds := fleetfile.Validate(ctx, doc, root)
	ok := !diag.HasErrors(ds)
	sha, _ := fleetfile.RevParse(ctx, root)
	plain := "ok"
	if ok {
		plain = "ok " + doc.Metadata.Name + " " + doc.Metadata.Version + " " + sha
	}
	return g.report(diag.Report{
		OK:              ok,
		Diagnostics:     ds,
		Name:            doc.Metadata.Name,
		Path:            root,
		GitSHA:          sha,
		TopologyVersion: doc.Metadata.Version,
		PlainOK:         plain,
	})
}
