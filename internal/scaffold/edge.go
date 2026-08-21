package scaffold

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DSamuelHodge/eve-fleet/internal/fleetfile"
)

var ErrEdgeExists = errors.New("edge tool already exists")

func EdgeTool(root string, edge fleetfile.EdgeSpec) error {
	dir := filepath.Join(root, "agents", edge.From, "agent", "tools", "fleet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "edge_"+edge.Name+".ts")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%w: %s", ErrEdgeExists, filepath.Join("agents", edge.From, "agent", "tools", "fleet", "edge_"+edge.Name+".ts"))
		}
		return err
	}
	_, werr := f.Write([]byte(edgeToolTS(edge)))
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
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
