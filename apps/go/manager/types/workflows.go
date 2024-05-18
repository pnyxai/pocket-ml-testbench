package types

type TestsData struct {
	Framework string   `json:"framework"`
	Tasks     []string `json:"tasks"`
}

type NodeManagerParams struct {
	Service       string      `json:"service"`
	SessionHeight int64       `json:"session_height"`
	Tests         []TestsData `json:"tests"`
}

type NodeManagerResults struct {
	Success  uint `json:"success"`
	Failed   uint `json:"failed"`
	NewNodes uint `json:"new_nodes"`
}

type NodeAnalysisChanResponse struct {
	Request  *NodeData
	Response *AnalyzeNodeResults
}

type SamplerWorkflowParams struct {
	Framework     string        `json:"framework"`
	Task          string        `json:"tasks"`
	RequesterArgs RequesterArgs `json:"requester_args"`
	Blacklist     []int         `json:"blacklist"`
	Qty           int           `json:"qty"`
}
