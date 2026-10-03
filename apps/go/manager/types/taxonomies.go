package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TaxonomyScalarStats mirrors ScalarStatsRecord on the summarizer side.
type TaxonomyScalarStats struct {
	Mean   float64 `bson:"mean"`
	Median float64 `bson:"median"`
	Std    float64 `bson:"std"`
	N      int64   `bson:"n"`
}

// TaxonomyVectorStats mirrors VectorStatsRecord on the summarizer side.
type TaxonomyVectorStats struct {
	Mean   []*float64 `bson:"mean"`
	Median []*float64 `bson:"median"`
	Std    []*float64 `bson:"std"`
	N      []int64    `bson:"n"`
}

// TaxonomyStats groups the aggregated statistics of a taxonomy node.
type TaxonomyStats struct {
	Score TaxonomyScalarStats `bson:"score"`
	Time  TaxonomyScalarStats `bson:"time"`
	Cost  TaxonomyVectorStats `bson:"cost"`
}

type TaxonomyNode struct {
	Stats     TaxonomyStats `bson:"stats"`
	SampleMin int64         `bson:"sample_min"`
}

type TaxonomySummary struct {
	SupplierID          primitive.ObjectID      `bson:"supplier_id"`
	SummaryDate         time.Time               `bson:"summary_date"`
	TaxonomyName        string                  `bson:"taxonomy_name"`
	TaxonomyNodesScores map[string]TaxonomyNode `bson:"taxonomy_nodes_scores"`
}
