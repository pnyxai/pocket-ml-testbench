package types

import "go.mongodb.org/mongo-driver/bson/primitive"

type TestsData struct {
	Framework string   `json:"framework"`
	Tasks     []string `json:"tasks"`
}

type SupplierManagerParams struct {
	Service       string      `json:"service"`
	SessionHeight int64       `json:"session_height"`
	Tests         []TestsData `json:"tests"`
}

type SupplierManagerResults struct {
	SuccessSuppliers uint `json:"success"`
	FailedSuppliers  uint `json:"failed"`
	NewSuppliers     uint `json:"new_suppliers"`
	TriggeredTasks   uint `json:"triggered_tasks"`
}

type SupplierAnalysisChanResponse struct {
	Request  *SupplierData
	Response *AnalyzeSupplierResults
}

type SamplerWorkflowParams struct {
	Framework     string        `json:"framework"`
	Task          string        `json:"tasks"`
	RequesterArgs RequesterArgs `json:"requester_args"`
	Blacklist     []int         `json:"blacklist"`
	Qty           int           `json:"qty"`
}

type ResultAnalyzerParams struct {
	TaskID primitive.ObjectID `json:"task_id"`
}

type ResultAnalyzerResults struct {
	Success bool `json:"success"`
}
