package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newReloadCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "reload",
		Short:   "Hot-load implementation trees only",
		GroupID: groupOperate,
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runReload(cmd.Context())
		},
	}
}

func (g *globals) runReload(ctx context.Context) error {
	if g.Agent == "" && !g.Shared {
		return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".", "reload.target",
				"reload requires --agent and/or --shared",
				"pass --agent=<name> or --shared"),
		}})
	}
	root, current, _, _, err := g.findFleet(ctx)
	if err != nil {
		return err
	}
	pin, err := readPin(root)
	if err != nil {
		return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".eve-fleet/deploy.json", "reload.pin",
				"no deployed revision to diff against",
				"run eve-fleet deploy first"),
		}})
	}
	ctx, cancel := context.WithTimeout(ctx, fleetfile.GitTimeout)
	defer cancel()
	raw, err := fleetfile.ShowFile(ctx, root, pin.GitSHA, fleetfile.FileName)
	if err != nil {
		return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error("Fleetfile", "reload.revision",
				fmt.Sprintf("cannot read Fleetfile at %s: %v", pin.GitSHA, err),
				"deploy a real git SHA"),
		}})
	}
	deployed, err := decodeDocument(raw)
	if err != nil {
		return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error("Fleetfile", "fleetfile.parse", err.Error(), "fix YAML at the deployed revision"),
		}})
	}
	head, _ := fleetfile.RevParse(ctx, root)
	if head == "" {
		head = "worktree"
	}
	changed, why := fleetfile.TopologyChanged(deployed, current)
	if changed {
		_ = fleetfile.AppendAudit(root, fleetfile.AuditEvent{
			Kind:            "reload.refused",
			Fleet:           current.Metadata.Name,
			TopologyVersion: current.Metadata.Version,
			GitSHA:          head,
			FromSHA:         pin.GitSHA,
			ToSHA:           head,
			Actor:           g.Agent,
			Status:          "refused",
			Message:         why,
		})
		return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error("Fleetfile", "reload.topology",
				fmt.Sprintf("%s (from %s to %s)", why, pin.GitSHA, head),
				"commit the topology change and deploy a new revision; reload only hot-loads implementation trees"),
		}})
	}
	target := "shared/"
	if g.Agent != "" {
		target = "agents/" + g.Agent + "/agent/"
		if _, ok := current.Agents[g.Agent]; !ok {
			return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
				diag.Error("agents."+g.Agent, "reload.agent",
					fmt.Sprintf("agent %q is not in this fleet", g.Agent),
					"pass --agent of a declared agent"),
			}})
		}
	}
	_ = fleetfile.AppendAudit(root, fleetfile.AuditEvent{
		Kind:            "reload",
		Fleet:           current.Metadata.Name,
		TopologyVersion: current.Metadata.Version,
		GitSHA:          pin.GitSHA,
		FromSHA:         pin.GitSHA,
		ToSHA:           pin.GitSHA,
		Actor:           g.Agent,
		Status:          "ok",
		Message:         "hot-loaded " + target,
	})
	plain := fmt.Sprintf("reloaded %s at %s", target, pin.GitSHA)
	if g.Shared {
		plain += " (including shared/)"
	}
	return g.report(diag.Report{
		OK:              true,
		Name:            current.Metadata.Name,
		Path:            root,
		GitSHA:          pin.GitSHA,
		TopologyVersion: current.Metadata.Version,
		PlainOK:         plain,
	})
}

func (g *globals) findFleet(ctx context.Context) (root string, doc *fleetfile.Document, raw []byte, sha string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, nil, "", g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".", "runtime.cwd", err.Error(), "run the command from a readable directory"),
		}})
	}
	root, err = fleetfile.FindRoot(cwd)
	if err != nil {
		return "", nil, nil, "", g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(".", "fleetfile.missing", "no Fleetfile found in this directory or its parents", "run eve-fleet init <name>"),
		}})
	}
	doc, raw, err = fleetfile.Load(root)
	if err != nil {
		return "", nil, nil, "", g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error("Fleetfile", "fleetfile.parse", err.Error(), "fix YAML syntax in Fleetfile"),
		}})
	}
	return root, doc, raw, sha, nil
}

func readPin(root string) (pinRecord, error) {
	var rec pinRecord
	b, err := os.ReadFile(filepath.Join(root, ".eve-fleet", "deploy.json"))
	if err != nil {
		return rec, err
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		return rec, err
	}
	if rec.GitSHA == "" {
		return rec, fmt.Errorf("empty gitSHA")
	}
	return rec, nil
}

func decodeDocument(raw []byte) (*fleetfile.Document, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var doc fleetfile.Document
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}
