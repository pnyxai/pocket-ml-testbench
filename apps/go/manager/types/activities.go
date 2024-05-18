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
	Success  bool          `json:"success"`
	IsNew    bool          `json:"is_new"`
	Triggers []TaskTrigger `json:"task_trigger"`
}

type TaskTrigger struct {
	Address   string `bson:"address"`
	Service   string `bson:"service"`
	Framework string `bson:"framework"`
	Task      string `bson:"task"`
	Blacklist []int  `bson:"blacklist"`
	Qty       int    `bson:"qty"`
}

//------------------------------------------------------------------------------
// Trigger Sampler
//------------------------------------------------------------------------------

type TriggerSamplerParams struct {
	Trigger TaskTrigger
}

type TriggerSamplerResults struct {
	Success bool
}
