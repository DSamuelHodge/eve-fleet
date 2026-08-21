package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
	"github.com/spf13/cobra"
)

func newBuildCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "build",
		Short:   "Validate and project deployable artifacts",
		GroupID: groupOperate,
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runBuild(cmd.Context())
		},
	}
}

type buildManifest struct {
	GitSHA          string   `json:"gitSHA"`
	TopologyVersion string   `json:"topologyVersion"`
	Name            string   `json:"name"`
	Agents          []string `json:"agents"`
	Edges           []string `json:"edges"`
}

func (g *globals) runBuild(ctx context.Context) error {
	root, doc, ds, sha, err := g.loadValidated(ctx)
	if err != nil {
		return err
	}
	if diag.HasErrors(ds) {
		return g.report(diag.Report{OK: false, Diagnostics: ds})
	}
	agents := make([]string, 0, len(doc.Agents))
	for name := range doc.Agents {
		agents = append(agents, name)
	}
	sort.Strings(agents)
	edges := make([]string, 0, len(doc.Edges))
	for _, e := range doc.Edges {
		edges = append(edges, e.Name)
	}
	sort.Strings(edges)
	man := buildManifest{
		GitSHA:          sha,
		TopologyVersion: doc.Metadata.Version,
		Name:            doc.Metadata.Name,
		Agents:          agents,
		Edges:           edges,
	}
	dir := filepath.Join(root, ".eve-fleet", "build", sha)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return g.report(diag.Report{OK: false, Diagnostics: []diag.Diagnostic{
			diag.Error(dir, "build.dir", err.Error(), "ensure .eve-fleet is writable"),
		}})
	}
	body, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	out := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(out, body, 0o644); err != nil {
		return err
	}
	return g.report(diag.Report{
		OK:              true,
		Name:            doc.Metadata.Name,
		Path:            root,
		GitSHA:          sha,
		TopologyVersion: doc.Metadata.Version,
		PlainOK:         fmt.Sprintf("built %s %s", doc.Metadata.Name, sha),
	})
}
