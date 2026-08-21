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
	rel := filepath.Join("agents", edge.From, "agent", "tools", "fleet", "edge_"+edge.Name+".ts")
	if err := writeExclusive(path, rel, []byte(edgeToolTS(edge))); err != nil {
		return err
	}
	if !edge.RequiresAck {
		return nil
	}
	if err := AckRejectTools(root, edge); err != nil {
		_ = os.Remove(path)
		dir := filepath.Join(root, "agents", edge.From, "agent", "tools", "fleet")
		_ = os.Remove(filepath.Join(dir, "ack_edge_"+edge.Name+".ts"))
		_ = os.Remove(filepath.Join(dir, "reject_edge_"+edge.Name+".ts"))
		return err
	}
	return nil
}

func AckRejectTools(root string, edge fleetfile.EdgeSpec) error {
	dir := filepath.Join(root, "agents", edge.From, "agent", "tools", "fleet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ackRel := filepath.Join("agents", edge.From, "agent", "tools", "fleet", "ack_edge_"+edge.Name+".ts")
	if err := writeExclusive(filepath.Join(dir, "ack_edge_"+edge.Name+".ts"), ackRel, []byte(ackToolTS(edge))); err != nil {
		return err
	}
	rejRel := filepath.Join("agents", edge.From, "agent", "tools", "fleet", "reject_edge_"+edge.Name+".ts")
	return writeExclusive(filepath.Join(dir, "reject_edge_"+edge.Name+".ts"), rejRel, []byte(rejectToolTS(edge)))
}

func writeExclusive(path, rel string, body []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%w: %s", ErrEdgeExists, rel)
		}
		return err
	}
	_, werr := f.Write(body)
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

func ackToolTS(edge fleetfile.EdgeSpec) string {
	desc := fmt.Sprintf("Acknowledge completed edge %q. Callable only after edge_%s has completed. Optional reason. Timeout without ack/reject fails the edge and the parent outcome.", edge.Name, edge.Name)
	return ackRejectBody(desc, "acked")
}

func rejectToolTS(edge fleetfile.EdgeSpec) string {
	desc := fmt.Sprintf("Reject completed edge %q. Callable only after edge_%s has completed. Optional reason. Reject fails the parent outcome.", edge.Name, edge.Name)
	return ackRejectBody(desc, "rejected")
}

func ackRejectBody(desc, action string) string {
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
    reason: z.string().optional(),
  }),
  async execute(input) {
    return { status: "ok" as const, result: { action: %q, reason: input.reason } };
  },
});
`, descLit, action)
}

func edgeToolTS(edge fleetfile.EdgeSpec) string {
	onf := edge.OnFailure
	if onf == "" {
		onf = "parent_handles"
	}
	desc := fmt.Sprintf("Handoff edge %q from %s to %s. Contract: %s. on_failure=%s (retry without a supervisor degrades to parent_handles). Timeout without ack/reject fails the edge and the parent outcome.", edge.Name, edge.From, edge.To, edge.Contract, onf)
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
