package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
	"github.com/spf13/cobra"
)

func newAuditCmd(g *globals) *cobra.Command {
	var outcome, run string
	cmd := &cobra.Command{
		Use:     "audit",
		Short:   "Reconstruct the accountability chain",
		GroupID: groupInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runAudit(cmd.Context(), outcome, run)
		},
	}
	cmd.Flags().StringVar(&outcome, "outcome", "", "filter by outcome id")
	cmd.Flags().StringVar(&run, "run", "", "filter by run id")
	return cmd
}

func (g *globals) runAudit(ctx context.Context, outcome, run string) error {
	root, doc, _, _, err := g.findFleet(context.Background())
	if err != nil {
		return err
	}
	events, err := fleetfile.ReadAudit(root)
	if err != nil {
		return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(fleetfile.AuditFile, "audit.read", err.Error(), "ensure .eve-fleet/audit.jsonl is readable"),
		}})
	}
	var chain []fleetfile.AuditEvent
	for _, ev := range events {
		if outcome != "" && ev.OutcomeID != outcome && ev.OutcomeID != "" {
			continue
		}
		if run != "" && ev.GitSHA != run && ev.FromSHA != run && ev.ToSHA != run {
			continue
		}
		if ev.PayloadHash == "" && ev.Message != "" {
			sum := sha256.Sum256([]byte(ev.Message))
			ev.PayloadHash = hex.EncodeToString(sum[:])
		}
		if ev.Fleet == "" {
			ev.Fleet = doc.Metadata.Name
		}
		if ev.TopologyVersion == "" {
			ev.TopologyVersion = doc.Metadata.Version
		}
		if ev.Actor == "subagent" {
			ev.Actor = "owning-agent"
			ev.Message = "subagent activity collapsed under owning agent"
		}
		chain = append(chain, ev)
	}
	if g.JSON {
		type payload struct {
			OK              bool                   `json:"ok"`
			Name            string                 `json:"name"`
			GitSHA          string                 `json:"gitSHA,omitempty"`
			TopologyVersion string                 `json:"topologyVersion,omitempty"`
			Events          []fleetfile.AuditEvent `json:"events"`
			Note            string                 `json:"note,omitempty"`
		}
		sha := ""
		if pin, err := readPin(root); err == nil {
			sha = pin.GitSHA
		}
		return g.writeJSON(payload{
			OK: true, Name: doc.Metadata.Name, GitSHA: sha,
			TopologyVersion: doc.Metadata.Version, Events: chain,
			Note: "parent owns the outcome even if intermediates were not inspected",
		})
	}
	plain := fmt.Sprintf("audit %s events=%d (parent owns the outcome; payloadHash not payload)",
		doc.Metadata.Name, len(chain))
	for _, ev := range chain {
		plain += fmt.Sprintf("\n%s %s %s->%s %s", ev.Kind, ev.EdgeName, ev.FromSHA, ev.ToSHA, ev.Status)
	}
	return g.report(diag.Report{
		OK:              true,
		Name:            doc.Metadata.Name,
		Path:            root,
		TopologyVersion: doc.Metadata.Version,
		PlainOK:         plain,
	})
}

func (g *globals) writeJSON(v any) error {
	enc := json.NewEncoder(g.stdout)
	return enc.Encode(v)
}
