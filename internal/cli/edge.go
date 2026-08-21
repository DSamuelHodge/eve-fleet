package cli

import (
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
		name     string
		from     string
		to       string
		contract string
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a named handoff edge",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runEdgeAdd(name, from, to, contract)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "unique edge name")
	cmd.Flags().StringVar(&from, "from", "", "caller agent")
	cmd.Flags().StringVar(&to, "to", "", "callee agent")
	cmd.Flags().StringVar(&contract, "contract", "", "free-text handoff contract")
	return cmd
}

func (g *globals) runEdgeAdd(name, from, to, contract string) error {
	edge := fleetfile.EdgeSpec{Name: name, From: from, To: to, Contract: contract}
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
	if ds := fleetfile.ValidateEdges(doc.Agents, proposed); len(ds) > 0 {
		return g.report(diag.Report{OK: false, Diagnostics: ds})
	}
	if err := scaffold.EdgeTool(root, edge); err != nil {
		return err
	}
	doc.Edges = proposed
	if err := fleetfile.Save(root, doc); err != nil {
		_ = os.Remove(filepathJoinEdge(root, edge))
		return err
	}
	return g.report(diag.Report{
		OK:      true,
		Name:    name,
		Path:    root,
		PlainOK: fmt.Sprintf("Added edge %s (%s -> %s)", name, from, to),
	})
}

func filepathJoinEdge(root string, edge fleetfile.EdgeSpec) string {
	return filepath.Join(root, "agents", edge.From, "agent", "tools", "fleet", "edge_"+edge.Name+".ts")
}
