package diag

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Diagnostic struct {
	Path       string `json:"path"`
	Rule       string `json:"rule"`
	Level      string `json:"level"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}

func Error(path, rule, message, suggestion string) Diagnostic {
	return Diagnostic{Path: path, Rule: rule, Level: "error", Message: message, Suggestion: suggestion}
}

func HasErrors(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Level == "error" {
			return true
		}
	}
	return false
}

type Report struct {
	OK              bool         `json:"ok"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
	Name            string       `json:"name,omitempty"`
	Path            string       `json:"path,omitempty"`
	GitSHA          string       `json:"gitSHA,omitempty"`
	TopologyVersion string       `json:"topologyVersion,omitempty"`
}

func (r Report) WriteJSON(w io.Writer) error {
	if r.Diagnostics == nil {
		r.Diagnostics = []Diagnostic{}
	}
	enc := json.NewEncoder(w)
	return enc.Encode(r)
}

func (r Report) WritePlain(w io.Writer) error {
	if r.OK && len(r.Diagnostics) == 0 {
		if r.Name != "" && r.GitSHA != "" {
			_, err := fmt.Fprintf(w, "Initialized fleet %s (git SHA %s)\n", r.Name, r.GitSHA)
			return err
		}
		if r.Name != "" {
			_, err := fmt.Fprintf(w, "ok %s %s %s\n", r.Name, r.TopologyVersion, r.GitSHA)
			return err
		}
		_, err := fmt.Fprintln(w, "ok")
		return err
	}
	var b strings.Builder
	for _, d := range r.Diagnostics {
		fmt.Fprintf(&b, "%s  %s  %s\n  %s\n  suggestion: %s\n", d.Level, d.Path, d.Rule, d.Message, d.Suggestion)
	}
	_, err := io.WriteString(w, b.String())
	return err
}
