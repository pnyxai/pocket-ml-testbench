package types

import "go.mongodb.org/mongo-driver/bson/primitive"

//------------------------------------------------------------------------------
// Get Staked Suppliers
//------------------------------------------------------------------------------

type GetStakedParams struct {
	Service string
}

type SupplierData struct {
	Address string
	Service string
}

type BlockData struct {
	Height           int64
	BlocksPerSession int64
}

type GetStakedResults struct {
	Suppliers []SupplierData
	Block     BlockData
}

//------------------------------------------------------------------------------
// Analyze Suppliers
//------------------------------------------------------------------------------

type AnalyzeSupplierParams struct {
	Supplier SupplierData `json:"supplier"`
	Block    BlockData    `json:"block"`
	Tests    []TestsData  `json:"tests"`
}

type AnalyzeSupplierResults struct {
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

//------------------------------------------------------------------------------
// Analyze Results
//------------------------------------------------------------------------------

type AnalyzeResultParams struct {
	TaskID primitive.ObjectID `json:"task_id"`
}

type AnalyzeResultResults struct {
	Success bool `json:"success"`
}
