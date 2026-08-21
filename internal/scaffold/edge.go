package scaffold

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
)

func EdgeTool(root string, edge fleetfile.EdgeSpec) error {
	dir := filepath.Join(root, "agents", edge.From, "agent", "tools", "fleet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "edge_"+edge.Name+".ts")
	return os.WriteFile(path, []byte(edgeToolTS(edge)), 0o644)
}

func edgeToolTS(edge fleetfile.EdgeSpec) string {
	desc := fmt.Sprintf("Handoff edge %q from %s to %s. Contract: %s", edge.Name, edge.From, edge.To, edge.Contract)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(desc); err != nil {
		buf.Reset()
		buf.WriteString(`"handoff"`)
	}
	descLit := bytes.TrimSpace(buf.Bytes())
	return fmt.Sprintf(`import { defineTool } from "eve/tools";
import { z } from "zod";

export default defineTool({
  description: %s,
  inputSchema: z.object({
    payload: z.unknown(),
    context: z.unknown().optional(),
  }),
  async execute(input) {
    return { status: "ok" as const, result: input.payload };
  },
});
`, descLit)
}
