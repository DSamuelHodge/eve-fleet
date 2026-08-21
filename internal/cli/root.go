package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

const (
	groupFleet   = "fleet"
	groupOperate = "operate"
	groupInspect = "inspect"
)

type globals struct {
	JSON           bool
	Yes            bool
	NonInteractive bool
	Revision       string
	Agent          string
	Shared         bool
	stdout         io.Writer
	stderr         io.Writer
	stdin          io.Reader
}

func newRoot(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "eve-fleet",
		Short:         "Convention layer for Vercel Eve agent fleets",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.PersistentFlags().BoolVar(&g.JSON, "json", false, "machine-readable JSON output")
	cmd.PersistentFlags().BoolVar(&g.Yes, "yes", false, "assume yes for prompts")
	cmd.PersistentFlags().BoolVar(&g.NonInteractive, "non-interactive", false, "never prompt")
	cmd.PersistentFlags().StringVar(&g.Revision, "revision", "", "git SHA to operate on")
	cmd.PersistentFlags().StringVar(&g.Agent, "agent", "", "agent name")
	cmd.PersistentFlags().BoolVar(&g.Shared, "shared", false, "include shared implementations")
	cmd.SetOut(g.stdout)
	cmd.SetErr(g.stderr)
	cmd.SetIn(g.stdin)

	cmd.AddGroup(
		&cobra.Group{ID: groupFleet, Title: "Fleet Setup"},
		&cobra.Group{ID: groupOperate, Title: "Operate"},
		&cobra.Group{ID: groupInspect, Title: "Inspect"},
	)

	cmd.AddCommand(newVersionCmd(g))
	cmd.AddCommand(newInitCmd(g))
	cmd.AddCommand(newDoctorCmd(g))
	cmd.AddCommand(newAgentCmd(g))
	cmd.AddCommand(newEdgeCmd(g))
	cmd.AddCommand(newSharedCmd(g))
	cmd.AddCommand(newDevCmd(g))
	cmd.AddCommand(newBuildCmd(g))
	cmd.AddCommand(newLinkCmd(g))
	cmd.AddCommand(newDeployCmd(g))
	cmd.AddCommand(newStubCmd(g, "reload", "Hot-load implementation trees only", "", groupOperate))
	cmd.AddCommand(newStatusCmd(g))
	cmd.AddCommand(newStubCmd(g, "audit", "Reconstruct the accountability chain", "", groupInspect))
	return cmd
}

func newVersionCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Print the eve-fleet CLI version",
		GroupID: groupInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			if g.JSON {
				_, err := fmt.Fprintf(g.stdout, `{"version":%q}`+"\n", version)
				return err
			}
			_, err := fmt.Fprintln(g.stdout, version)
			return err
		},
	}
	return cmd
}

func newStubCmd(g *globals, use, short, child, group string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		GroupID: group,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("%s is not implemented yet", use)
		},
	}
	if child != "" {
		sub := &cobra.Command{
			Use:   child,
			Short: short,
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("%s %s is not implemented yet", use, child)
			},
		}
		cmd.AddCommand(sub)
		cmd.RunE = nil
	}
	return cmd
}

func Execute(args []string) int {
	g := &globals{
		stdout: os.Stdout,
		stderr: os.Stderr,
		stdin:  os.Stdin,
	}
	root := newRoot(g)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		if errors.Is(err, errPrinted) {
			return 1
		}
		fmt.Fprintln(g.stderr, err.Error())
		return 1
	}
	return 0
}
