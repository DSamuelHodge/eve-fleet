package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/spf13/cobra"
)

func newLinkCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "link",
		Short:   "Link a fleet deployment",
		GroupID: groupOperate,
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runPin(cmd.Context(), "link")
		},
	}
}

func newDeployCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "deploy",
		Short:   "Deploy a fleet revision",
		GroupID: groupOperate,
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runPin(cmd.Context(), "deploy")
		},
	}
}

type pinRecord struct {
	Kind            string `json:"kind"`
	GitSHA          string `json:"gitSHA"`
	TopologyVersion string `json:"topologyVersion"`
	Name            string `json:"name"`
	RecordedAt      string `json:"recordedAt"`
}

func (g *globals) runPin(ctx context.Context, kind string) error {
	root, doc, ds, sha, err := g.loadValidated(ctx)
	if err != nil {
		return err
	}
	if diag.HasErrors(ds) {
		return g.report(diag.Report{OK: false, Diagnostics: ds})
	}
	if g.Revision != "" && g.Revision != sha {
		return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".", "deploy.revision",
				fmt.Sprintf("--revision %s does not match HEAD %s", g.Revision, sha),
				"check out that commit or omit --revision to pin HEAD"),
		}})
	}
	rec := pinRecord{
		Kind:            kind,
		GitSHA:          sha,
		TopologyVersion: doc.Metadata.Version,
		Name:            doc.Metadata.Name,
		RecordedAt:      time.Now().UTC().Format(time.RFC3339),
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
	path := filepath.Join(dir, "deploy.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return err
	}
	return g.report(diag.Report{
		OK:              true,
		Name:            doc.Metadata.Name,
		Path:            root,
		GitSHA:          sha,
		TopologyVersion: doc.Metadata.Version,
		PlainOK:         fmt.Sprintf("%s %s pinned to %s", kind, doc.Metadata.Name, sha),
	})
}
