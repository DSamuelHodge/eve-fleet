package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
	"github.com/spf13/cobra"
)

func newSharedCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "shared",
		Short:   "Register shared skills, tools, or connections",
		GroupID: groupFleet,
	}
	cmd.AddCommand(newSharedAddCmd(g))
	return cmd
}

func newSharedAddCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "add <skill|tool|connection> <name>",
		Short: "Register a shared capability in the Fleetfile",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return g.report(diag.Report{
					OK: false,
					Diagnostics: []diag.Diagnostic{
						diag.Error(".", "shared.args",
							"shared add requires a kind and a name",
							"run eve-fleet shared add skill|tool|connection <name>"),
					},
				})
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runSharedAdd(args[0], args[1])
		},
	}
}

func (g *globals) runSharedAdd(kind, name string) error {
	dir, field, errKind := sharedKind(kind)
	if errKind != "" {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error(".", "shared.kind",
					errKind,
					"use skill, tool, or connection"),
			},
		})
	}
	if !fleetfile.ValidDNSLabel(name) {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error("shared."+field, "shared.name",
					"shared name must be a DNS-label",
					"use a name like crm-write"),
			},
		})
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
	if doc.Shared == nil {
		doc.Shared = &fleetfile.Shared{}
	}
	list := sharedList(doc.Shared, field)
	for _, existing := range list {
		if existing == name {
			return g.report(diag.Report{
				OK: false,
				Diagnostics: []diag.Diagnostic{
					diag.Error("shared."+field, "shared.exists",
						fmt.Sprintf("%s %q is already registered", kind, name),
						"choose a new name"),
				},
			})
		}
	}
	setSharedList(doc.Shared, field, append(list, name))
	target := filepath.Join(root, "shared", dir, name)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error(filepath.Dir(target), "shared.dir.write", err.Error(),
					"ensure the fleet root is writable"),
			},
		})
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		if os.IsExist(err) {
			return g.report(diag.Report{
				OK: false,
				Diagnostics: []diag.Diagnostic{
					diag.Error(target, "shared.dir.exists",
						fmt.Sprintf("directory %s already exists", target),
						"choose a new name or remove the directory"),
				},
			})
		}
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error(target, "shared.dir.write", err.Error(),
					"ensure the fleet root is writable"),
			},
		})
	}
	if err := fleetfile.Save(root, doc); err != nil {
		_ = os.RemoveAll(target)
		return g.report(diag.Report{
			OK: false,
			Diagnostics: []diag.Diagnostic{
				diag.Error(filepath.Join(root, fleetfile.FileName), "fleetfile.save",
					err.Error(),
					"fix permissions on Fleetfile and retry"),
			},
		})
	}
	plain := fmt.Sprintf("Registered shared %s %s", kind, name)
	if kind == "tool" {
		plain += " (" + g.sharedToolApproval(doc, name) + ")"
	}
	return g.report(diag.Report{
		OK:      true,
		Name:    name,
		Path:    root,
		PlainOK: plain,
	})
}

func (g *globals) sharedToolApproval(doc *fleetfile.Document, name string) string {
	if g.Agent == "" {
		return "inherits the calling agent's approval policy at use time"
	}
	if doc.Agents == nil {
		return fmt.Sprintf("inherits %s approval policy at use time", g.Agent)
	}
	spec, ok := doc.Agents[g.Agent]
	if !ok || spec.ApprovalPolicy == nil {
		return fmt.Sprintf("inherits %s approval policy at use time", g.Agent)
	}
	approver := spec.ApprovalPolicy.Approver
	timeout := spec.ApprovalPolicy.Timeout
	if override, ok := spec.ApprovalPolicy.Tools[name]; ok {
		if override.Approver != "" {
			approver = override.Approver
		}
		if override.Timeout != "" {
			timeout = override.Timeout
		}
	}
	msg := fmt.Sprintf("caller %s approver=%s", g.Agent, approver)
	if timeout != "" {
		msg += " timeout=" + timeout
	}
	return msg
}

func sharedKind(kind string) (dir, field, err string) {
	switch kind {
	case "skill":
		return "skills", "skills", ""
	case "tool":
		return "tools", "tools", ""
	case "connection":
		return "connections", "connections", ""
	default:
		return "", "", `kind must be "skill", "tool", or "connection"`
	}
}

func sharedList(s *fleetfile.Shared, field string) []string {
	switch field {
	case "skills":
		return s.Skills
	case "tools":
		return s.Tools
	case "connections":
		return s.Connections
	}
	return nil
}

func setSharedList(s *fleetfile.Shared, field string, v []string) {
	switch field {
	case "skills":
		s.Skills = v
	case "tools":
		s.Tools = v
	case "connections":
		s.Connections = v
	}
}
