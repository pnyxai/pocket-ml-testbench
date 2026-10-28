package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TaxonomyScalarStats holds the aggregated statistics of a taxonomy node.
// SE is the standard error of the reported (unweighted) mean and VEff is the
// Welch-Satterthwaite effective degrees of freedom.
type TaxonomyScalarStats struct {
	Mean   float64 `bson:"mean"`
	Median float64 `bson:"median"`
	SE     float64 `bson:"se"`
	VEff   float64 `bson:"v_eff"`
}

// TaxonomyVectorStats holds the aggregated per-position statistics of a
// taxonomy node. See TaxonomyScalarStats for the SE/VEff semantics.
type TaxonomyVectorStats struct {
	Mean   []*float64 `bson:"mean"`
	Median []*float64 `bson:"median"`
	SE     []*float64 `bson:"se"`
	VEff   []*float64 `bson:"v_eff"`
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
