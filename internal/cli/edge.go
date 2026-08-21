package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
	"github.com/DSamuelHodge/eve-fleet/internal/scaffold"
	"github.com/spf13/cobra"
)

func newEdgeCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edge",
		Short:   "Add and inspect named edges",
		GroupID: groupFleet,
	}
	cmd.AddCommand(newEdgeAddCmd(g))
	return cmd
}

func newEdgeAddCmd(g *globals) *cobra.Command {
	var (
		name        string
		from        string
		to          string
		contract    string
		timeout     string
		onFailure   string
		requiresAck bool
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a named handoff edge",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runEdgeAdd(fleetfile.EdgeSpec{
				Name: name, From: from, To: to, Contract: contract,
				Timeout: timeout, OnFailure: onFailure, RequiresAck: requiresAck,
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "unique edge name")
	cmd.Flags().StringVar(&from, "from", "", "caller agent")
	cmd.Flags().StringVar(&to, "to", "", "callee agent")
	cmd.Flags().StringVar(&contract, "contract", "", "free-text handoff contract")
	cmd.Flags().StringVar(&timeout, "timeout", "", "optional edge timeout (e.g. 15m)")
	cmd.Flags().StringVar(&onFailure, "on-failure", "", "parent_handles, retry, escalate, or fail")
	cmd.Flags().BoolVar(&requiresAck, "requires-ack", false, "parent must ack or reject after the edge completes")
	return cmd
}

func (g *globals) runEdgeAdd(edge fleetfile.EdgeSpec) error {
	name, from, to := edge.Name, edge.From, edge.To
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
				diag.Error("Fleetfile", "fleetfile.parse", err.Error(), "fix YAML syntax in Fleetfile"),
			},
		})
	}
	if doc.Edges == nil {
		doc.Edges = []fleetfile.EdgeSpec{}
	}
	proposed := append(append([]fleetfile.EdgeSpec{}, doc.Edges...), edge)
	probe := *doc
	probe.Edges = proposed
	if ds := fleetfile.ValidateEdges(&probe); diag.HasErrors(ds) {
		return g.report(diag.Report{OK: false, Diagnostics: ds})
	}
	toolPath := filepathJoinEdge(root, edge)
	if err := scaffold.EdgeTool(root, edge); err != nil {
		rule := "edge.tool.write"
		if errors.Is(err, scaffold.ErrEdgeExists) {
			rule = "edge.tool.exists"
		}
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error(toolPath, rule, err.Error(),
					"remove the existing generated tool or choose a new edge name"),
			},
		})
	}
	doc.Edges = proposed
	if err := fleetfile.Save(root, doc); err != nil {
		removeEdgeArtifacts(root, edge)
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error(filepath.Join(root, fleetfile.FileName), "fleetfile.save",
					err.Error(),
					"fix permissions on Fleetfile and retry"),
			},
		})
	}
	plain := fmt.Sprintf("Added edge %s (%s -> %s)", name, from, to)
	if edge.RequiresAck {
		plain += " with ack/reject on the parent"
	}
	return g.report(diag.Report{
		OK:      true,
		Name:    name,
		Path:    root,
		PlainOK: plain,
	})
}

func removeEdgeArtifacts(root string, edge fleetfile.EdgeSpec) {
	dir := filepath.Join(root, "agents", edge.From, "agent", "tools", "fleet")
	_ = os.Remove(filepath.Join(dir, "edge_"+edge.Name+".ts"))
	_ = os.Remove(filepath.Join(dir, "ack_edge_"+edge.Name+".ts"))
	_ = os.Remove(filepath.Join(dir, "reject_edge_"+edge.Name+".ts"))
}

func filepathJoinEdge(root string, edge fleetfile.EdgeSpec) string {
	return filepath.Join(root, "agents", edge.From, "agent", "tools", "fleet", "edge_"+edge.Name+".ts")
}
