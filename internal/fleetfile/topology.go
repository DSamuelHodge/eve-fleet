package fleetfile

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

type topoView struct {
	Agents     map[string]topoAgent `json:"agents"`
	Edges      []EdgeSpec           `json:"edges"`
	Shared     *Shared              `json:"shared"`
	Supervisor bool                 `json:"supervisor"`
	Isolation  string               `json:"isolation"`
}

type topoAgent struct {
	Path           string          `json:"path"`
	Role           string          `json:"role"`
	Owns           Owns            `json:"owns"`
	ApprovalPolicy *ApprovalPolicy `json:"approval_policy"`
}

func TopologyView(doc *Document) topoView {
	v := topoView{
		Agents: map[string]topoAgent{},
		Edges:  nil,
	}
	if doc == nil {
		return v
	}
	for name, spec := range doc.Agents {
		v.Agents[name] = topoAgent{
			Path:           spec.Path,
			Role:           spec.Role,
			Owns:           spec.Owns,
			ApprovalPolicy: spec.ApprovalPolicy,
		}
	}
	if doc.Edges != nil {
		v.Edges = append([]EdgeSpec{}, doc.Edges...)
		sort.Slice(v.Edges, func(i, j int) bool { return v.Edges[i].Name < v.Edges[j].Name })
	}
	v.Shared = doc.Shared
	if doc.Runtime != nil {
		v.Supervisor = doc.Runtime.Supervisor
		v.Isolation = doc.Runtime.Isolation
	}
	return v
}

func TopologyChanged(deployed, current *Document) (bool, string) {
	a, _ := json.Marshal(TopologyView(deployed))
	b, _ := json.Marshal(TopologyView(current))
	if reflect.DeepEqual(a, b) {
		return false, ""
	}
	return true, fmt.Sprintf("topology differs from deployed revision")
}
