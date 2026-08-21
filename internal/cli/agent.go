package cli

import (
	"fmt"
	"os"

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
		role        string
		outcome     string
		sla         string
		completion  string
		job         string
		contract    string
		model       string
		description string
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
			return g.runAgentAdd(args[0], role, outcome, sla, completion, job, contract, model, description)
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
	return cmd
}

func (g *globals) runAgentAdd(name, role, outcome, sla, completion, job, contract, model, description string) error {
	if !fleetfile.ValidDNSLabel(name) {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error("metadata.name", "agent.name",
					"agent name must be a DNS-label (lowercase letters, digits, hyphens)",
					"use a name like lead-intake"),
			},
		})
	}
	if role != "parent" && role != "delegate" {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error(".", "agent.role",
					`role must be "parent" or "delegate"`,
					"pass --role=parent or --role=delegate"),
			},
		})
	}
	if role == "parent" {
		if outcome == "" || sla == "" || completion == "" {
			return g.report(diag.Report{
				OK: false,
				Diagnostics: []diag.Diagnostic{
					diag.Error(".", "agent.owns.parent",
						"parent requires --outcome, --sla, and --completion",
						"pass those flags; --yes/--non-interactive never prompt"),
				},
			})
		}
		if job != "" || contract != "" {
			return g.report(diag.Report{
				OK: false,
				Diagnostics: []diag.Diagnostic{
					diag.Error(".", "agent.owns.role",
						"parent must not set --job or --contract",
						"omit job/contract for a parent"),
				},
			})
		}
	}
	if role == "delegate" {
		if job == "" || contract == "" {
			return g.report(diag.Report{
				OK: false,
				Diagnostics: []diag.Diagnostic{
					diag.Error(".", "agent.owns.delegate",
						"delegate requires --job and --contract",
						"pass those flags; --yes/--non-interactive never prompt"),
				},
			})
		}
		if outcome != "" || sla != "" || completion != "" {
			return g.report(diag.Report{
				OK: false,
				Diagnostics: []diag.Diagnostic{
					diag.Error(".", "agent.owns.role",
						"delegate must not set --outcome, --sla, or --completion",
						"omit outcome/sla/completion for a delegate"),
				},
			})
		}
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
	doc.Agents[name] = spec
	if err := fleetfile.Save(root, doc); err != nil {
		return err
	}
	if err := scaffold.AgentTree(root, name, spec); err != nil {
		return err
	}
	return g.report(diag.Report{
		OK:      true,
		Name:    name,
		Path:    root,
		PlainOK: fmt.Sprintf("Added %s agent %s", role, name),
	})
}
