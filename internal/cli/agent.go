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

func newAgentCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "agent",
		Short:   "Add and inspect fleet agents",
		GroupID: groupFleet,
	}
	cmd.AddCommand(newAgentAddCmd(g))
	return cmd
}

func newAgentAddCmd(g *globals) *cobra.Command {
	var (
		role            string
		outcome         string
		sla             string
		completion      string
		job             string
		contract        string
		model           string
		description     string
		approver        string
		approvalTimeout string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a parent or delegate agent",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return g.report(diag.Report{
					OK: false,
					Diagnostics: []diag.Diagnostic{
						diag.Error(".", "agent.name.required",
							"agent add requires a name",
							"run eve-fleet agent add <name> --role=parent|delegate"),
					},
				})
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runAgentAdd(args[0], role, outcome, sla, completion, job, contract, model, description, approver, approvalTimeout)
		},
	}
	cmd.Flags().StringVar(&role, "role", "", "parent or delegate")
	cmd.Flags().StringVar(&outcome, "outcome", "", "parent outcome")
	cmd.Flags().StringVar(&sla, "sla", "", "parent SLA")
	cmd.Flags().StringVar(&completion, "completion", "", "parent completion")
	cmd.Flags().StringVar(&job, "job", "", "delegate job")
	cmd.Flags().StringVar(&contract, "contract", "", "delegate contract")
	cmd.Flags().StringVar(&model, "model", "", "optional model id")
	cmd.Flags().StringVar(&description, "description", "", "optional description")
	cmd.Flags().StringVar(&approver, "approver", "", `approver agent name or "human"`)
	cmd.Flags().StringVar(&approvalTimeout, "approval-timeout", "", "optional approval timeout (e.g. 15m)")
	return cmd
}

func (g *globals) runAgentAdd(name, role, outcome, sla, completion, job, contract, model, description, approver, approvalTimeout string) error {
	spec := fleetfile.AgentSpec{
		Path:        fleetfile.AgentPath(name),
		Role:        role,
		Model:       model,
		Description: description,
		Owns: fleetfile.Owns{
			Outcome:    outcome,
			SLA:        sla,
			Completion: completion,
			Job:        job,
			Contract:   contract,
		},
	}
	if approver != "" || approvalTimeout != "" {
		spec.ApprovalPolicy = &fleetfile.ApprovalPolicy{Approver: approver, Timeout: approvalTimeout}
	}
	if ds := fleetfile.ValidateSpec(name, spec); len(ds) > 0 {
		return g.report(diag.Report{OK: false, Diagnostics: ds})
	}

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
	if doc.Agents == nil {
		doc.Agents = map[string]fleetfile.AgentSpec{}
	}
	if _, exists := doc.Agents[name]; exists {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error("agents."+name, "agent.exists",
					fmt.Sprintf("agent %s already exists", name),
					"choose a new name"),
			},
		})
	}
	if len(doc.Agents)+1 > fleetfile.MaxAgents {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error("agents", "fleetfile.agents.limit",
					fmt.Sprintf("at most %d agents", fleetfile.MaxAgents),
					"remove agents until the fleet has 50 or fewer"),
			},
		})
	}
	pending := map[string]fleetfile.AgentSpec{}
	for k, v := range doc.Agents {
		pending[k] = v
	}
	pending[name] = spec
	if ds := fleetfile.ValidateApproval(pending); diag.HasErrors(ds) {
		return g.report(diag.Report{OK: false, Diagnostics: ds})
	}
	if err := scaffold.AgentTree(root, name, spec); err != nil {
		if errors.Is(err, scaffold.ErrTreeExists) {
			return g.report(diag.Report{
				OK: false,
				Diagnostics: []diag.Diagnostic{
					diag.Error(filepath.Join("agents", name, "agent"), "agent.tree.exists",
						err.Error(),
						"remove the existing tree or pick a new agent name"),
				},
			})
		}
		return err
	}
	doc.Agents[name] = spec
	if err := fleetfile.Save(root, doc); err != nil {
		_ = os.RemoveAll(filepath.Join(root, "agents", name))
		return err
	}
	return g.report(diag.Report{
		OK:      true,
		Name:    name,
		Path:    root,
		PlainOK: fmt.Sprintf("Added %s agent %s", role, name),
	})
}
