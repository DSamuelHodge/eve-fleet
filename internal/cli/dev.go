package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
	"github.com/spf13/cobra"
)

func newDevCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "dev",
		Short:   "Run local multi-agent development",
		GroupID: groupOperate,
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runDev(cmd.Context())
		},
	}
}

type devRecord struct {
	Name            string   `json:"name"`
	GitSHA          string   `json:"gitSHA"`
	TopologyVersion string   `json:"topologyVersion"`
	Supervisor      bool     `json:"supervisor"`
	Degradation     []string `json:"degradation,omitempty"`
	Eve             string   `json:"eve"`
}

func (g *globals) runDev(ctx context.Context) error {
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
	doc, raw, err := fleetfile.Load(root)
	if err != nil {
		return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error("Fleetfile", "fleetfile.parse", err.Error(), "fix YAML syntax in Fleetfile"),
		}})
	}
	ctx, cancel := context.WithTimeout(ctx, fleetfile.GitTimeout)
	defer cancel()
	ds := fleetfile.Validate(ctx, doc, root, raw)
	if diag.HasErrors(ds) {
		return g.report(diag.Report{OK: false, Diagnostics: ds})
	}
	sha, _ := fleetfile.RevParse(ctx, root)
	supervisor := false
	if doc.Runtime != nil {
		supervisor = doc.Runtime.Supervisor
	}
	rec := devRecord{
		Name:            doc.Metadata.Name,
		GitSHA:          sha,
		TopologyVersion: doc.Metadata.Version,
		Supervisor:      supervisor,
		Eve:             "compose-only (stock eve; no fork)",
	}
	if !supervisor {
		rec.Degradation = []string{
			"timeouts are best-effort / audit-only",
			"retry degrades to parent_handles",
			"ack-before-close is best-effort / audit-only",
		}
	}
	dir := filepath.Join(root, ".eve-fleet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.WriteFile(filepath.Join(dir, "dev.json"), body, 0o644); err != nil {
		return err
	}
	plain := fmt.Sprintf("dev %s %s supervisor=%v", doc.Metadata.Name, sha, supervisor)
	if !supervisor {
		plain += " (timeouts, retry, ack-before-close best-effort / audit-only)"
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
