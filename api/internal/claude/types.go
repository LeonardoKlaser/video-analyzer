package claude

import "encoding/json"

// Result is the structured JSON we ask Claude to produce.
type Result struct {
	HookAnalysis struct {
		Score       int    `json:"score"`
		Why         string `json:"why"`
		Improvement string `json:"improvement"`
	} `json:"hook_analysis"`

	StructureAnalysis struct {
		FrameworkMatch  string   `json:"framework_match"`
		RetentionIssues []string `json:"retention_issues"`
	} `json:"structure_analysis"`

	VisualAnalysis struct {
		Rhythm         string `json:"rhythm"`
		FirstFrame     string `json:"first_frame"`
		DominantVisual string `json:"dominant_visual"`
	} `json:"visual_analysis"`

	KeyInsights        []string `json:"key_insights"`
	ActionItems        []string `json:"action_items"`
	ReplicationScript  string   `json:"replication_script,omitempty"`
	NeuromarketingRefs []string `json:"neuromarketing_refs,omitempty"`
	ViralElements      []string `json:"viral_elements,omitempty"`
	Verdict            string   `json:"verdict"`
	VerdictReason      string   `json:"verdict_reason"`
}

func (r *Result) AsRaw() (json.RawMessage, error) {
	return json.Marshal(r)
}
