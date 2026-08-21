package cli

import (
	"context"
	"fmt"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	root, doc, _, _, err := g.findFleet(ctx)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, fleetfile.GitTimeout)
	defer cancel()
	sha, _ := fleetfile.RevParse(ctx, root)
	supervisor := false
	if doc.Runtime != nil {
		supervisor = doc.Runtime.Supervisor
	}
	dirty, _ := fleetfile.Dirty(ctx, root)
	deployedSHA := ""
	drift := false
	if pin, err := readPin(root); err == nil {
		deployedSHA = pin.GitSHA
		if deployedSHA != sha {
			drift = true
		}
		if raw, err := fleetfile.ShowFile(ctx, root, pin.GitSHA, fleetfile.FileName); err == nil {
			var deployed fleetfile.Document
			if yaml.Unmarshal(raw, &deployed) == nil {
				if changed, _ := fleetfile.TopologyChanged(&deployed, doc); changed {
					drift = true
				}
			}
		}
	}
	if dirty {
		drift = true
	}
	plain := fmt.Sprintf("%s topology=%s git=%s deployed=%s drift=%v supervisor=%v",
		doc.Metadata.Name, doc.Metadata.Version, sha, deployedSHA, drift, supervisor)
	if !supervisor {
		plain += " degradation: timeouts, retry, ack-before-close are best-effort / audit-only"
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
