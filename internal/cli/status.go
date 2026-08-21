package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
	"github.com/spf13/cobra"
)

func newStatusCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show topology version, SHAs, drift, supervisor state",
		GroupID: groupInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runStatus(cmd.Context())
		},
	}
}

func (g *globals) runStatus(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := fleetfile.FindRoot(cwd)
	if err != nil {
		return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".", "fleetfile.missing", "no Fleetfile found in this directory or its parents", "run eve-fleet init <name>"),
		}})
	}
	doc, _, err := fleetfile.Load(root)
	if err != nil {
		return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error("Fleetfile", "fleetfile.parse", err.Error(), "fix YAML syntax in Fleetfile"),
		}})
	}
	ctx, cancel := context.WithTimeout(ctx, fleetfile.GitTimeout)
	defer cancel()
	sha, _ := fleetfile.RevParse(ctx, root)
	supervisor := false
	if doc.Runtime != nil {
		supervisor = doc.Runtime.Supervisor
	}
	plain := fmt.Sprintf("%s %s %s supervisor=%v", doc.Metadata.Name, doc.Metadata.Version, sha, supervisor)
	if !supervisor {
		plain += " degradation: timeouts, retry, ack-before-close are best-effort / audit-only"
	}
	if b, err := os.ReadFile(filepath.Join(root, ".eve-fleet", "dev.json")); err == nil {
		var rec struct {
			Degradation []string `json:"degradation"`
		}
		if json.Unmarshal(b, &rec) == nil && len(rec.Degradation) > 0 {
			plain += "\n" + strings.Join(rec.Degradation, "; ")
		}
	}
	return g.report(diag.Report{
		OK:              true,
		Name:            doc.Metadata.Name,
		Path:            root,
		GitSHA:          sha,
		TopologyVersion: doc.Metadata.Version,
		PlainOK:         plain,
	})
}
