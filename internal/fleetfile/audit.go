package fleetfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const AuditFile = ".eve-fleet/audit.jsonl"

type AuditEvent struct {
	Kind            string `json:"kind"`
	Fleet           string `json:"fleet,omitempty"`
	TopologyVersion string `json:"topologyVersion,omitempty"`
	GitSHA          string `json:"gitSHA,omitempty"`
	FromSHA         string `json:"fromSHA,omitempty"`
	ToSHA           string `json:"toSHA,omitempty"`
	EdgeName        string `json:"edgeName,omitempty"`
	From            string `json:"from,omitempty"`
	To              string `json:"to,omitempty"`
	Actor           string `json:"actor,omitempty"`
	PayloadHash     string `json:"payloadHash,omitempty"`
	Status          string `json:"status,omitempty"`
	Error           string `json:"error,omitempty"`
	RequiresAck     bool   `json:"requiresAck,omitempty"`
	Acked           bool   `json:"acked,omitempty"`
	AckedActor      string `json:"ackedActor,omitempty"`
	AckedAt         string `json:"ackedAt,omitempty"`
	Started         string `json:"started,omitempty"`
	Completed       string `json:"completed,omitempty"`
	OutcomeID       string `json:"outcomeId,omitempty"`
	At              string `json:"at,omitempty"`
	Message         string `json:"message,omitempty"`
}

func AppendAudit(root string, ev AuditEvent) error {
	if ev.At == "" {
		ev.At = time.Now().UTC().Format(time.RFC3339)
	}
	dir := filepath.Join(root, ".eve-fleet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(root, AuditFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(ev)
}

func ReadAudit(root string) ([]AuditEvent, error) {
	data, err := os.ReadFile(filepath.Join(root, AuditFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []AuditEvent
	for _, line := range splitJSONL(data) {
		var ev AuditEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	return out, nil
}

func splitJSONL(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
