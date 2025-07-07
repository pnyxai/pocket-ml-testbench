package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TaxonomyNode struct {
	Score      float64 `bson:"score"`
	ScoreDev   float64 `bson:"score_dev"`
	RunTime    float64 `bson:"run_time"`
	RunTimeDev float64 `bson:"run_time_dev"`
	SampleMin  int64   `bson:"sample_min"`
}

type TaxonomySummary struct {
	SupplierID          primitive.ObjectID      `bson:"supplier_id"`
	SummaryDate         time.Time               `bson:"summary_date"`
	TaxonomyName        string                  `bson:"taxonomy_name"`
	TaxonomyNodesScores map[string]TaxonomyNode `bson:"taxonomy_nodes_scores"`
}
