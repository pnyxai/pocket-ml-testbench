package types

//------------------------------------------------------------------------------
// Get Staked Nodes
//------------------------------------------------------------------------------

type GetStakedParams struct {
	Service string
}

type NodeData struct {
	Address string
	Service string
}

type GetStakedResults struct {
	Nodes []NodeData
}

//------------------------------------------------------------------------------
// Analyze Nodes
//------------------------------------------------------------------------------

type AnalyzeNodeParams struct {
	Node  NodeData    `json:"node"`
	Tests []TestsData `json:"tests"`
}

type AnalyzeNodeResults struct {
	Success bool     `json:"success"`
	Node    NodeData `json:"node"`
}
