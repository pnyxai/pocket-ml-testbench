package records

import (
	"math"
	"testing"
	"time"

	"manager/types"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func f64Ptr(v float64) *float64 { return &v }

func f32Ptr(v float32) *float32 { return &v }

func approxEqual(a, b float64) bool { return math.Abs(a-b) < 1e-5 }

func checkCostPtr(t *testing.T, name string, got *float32, want *float32) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("%s: got %v, want %v", name, got, want)
	}
	if got != nil && !approxEqual(float64(*got), float64(*want)) {
		t.Fatalf("%s: got %v, want %v", name, *got, *want)
	}
}

func TestComputeVectorStats(t *testing.T) {
	// Sample standard deviation (n-1), matching gonum/stat.StdDev used for the
	// score/time statistics.
	sampleStd3 := float32(math.Sqrt(18.0 / 2.0)) // [1,4,7] or [2,5,8]
	sampleStd45 := float32(math.Sqrt(4.5))       // [6,9]
	sampleStd2 := float32(math.Sqrt(2.0))        // [2,4]
	sampleStd05 := float32(math.Sqrt(0.5))       // [1,2]
	sampleStd50 := float32(math.Sqrt(50.0))      // [10,20]

	cases := []struct {
		name       string
		costs      [][]*float64
		wantMean   []*float32
		wantMedian []*float32
		wantStd    []*float32
		wantN      []uint32
	}{
		{
			name:       "empty",
			costs:      nil,
			wantMean:   []*float32{},
			wantMedian: []*float32{},
			wantStd:    []*float32{},
			wantN:      []uint32{},
		},
		{
			name:       "no valid rows",
			costs:      [][]*float64{{}, {}},
			wantMean:   []*float32{},
			wantMedian: []*float32{},
			wantStd:    []*float32{},
			wantN:      []uint32{},
		},
		{
			name:       "single vector has zero deviation",
			costs:      [][]*float64{{f64Ptr(1), f64Ptr(2), f64Ptr(3)}},
			wantMean:   []*float32{f32Ptr(1), f32Ptr(2), f32Ptr(3)},
			wantMedian: []*float32{f32Ptr(1), f32Ptr(2), f32Ptr(3)},
			wantStd:    []*float32{f32Ptr(0), f32Ptr(0), f32Ptr(0)},
			wantN:      []uint32{1, 1, 1},
		},
		{
			name: "null skips per position",
			costs: [][]*float64{
				{f64Ptr(1), f64Ptr(2), nil},
				{f64Ptr(4), f64Ptr(5), f64Ptr(6)},
				{f64Ptr(7), f64Ptr(8), f64Ptr(9)},
			},
			wantMean:   []*float32{f32Ptr(4), f32Ptr(5), f32Ptr(7.5)},
			wantMedian: []*float32{f32Ptr(4), f32Ptr(5), f32Ptr(7.5)},
			wantStd:    []*float32{&sampleStd3, &sampleStd3, &sampleStd45},
			wantN:      []uint32{3, 3, 2},
		},
		{
			name: "all null position stays null",
			costs: [][]*float64{
				{nil, f64Ptr(2)},
				{nil, f64Ptr(4)},
			},
			wantMean:   []*float32{nil, f32Ptr(3)},
			wantMedian: []*float32{nil, f32Ptr(3)},
			wantStd:    []*float32{nil, &sampleStd2},
			wantN:      []uint32{0, 2},
		},
		{
			name: "ragged vectors",
			costs: [][]*float64{
				{f64Ptr(1)},
				{f64Ptr(2), f64Ptr(3)},
			},
			wantMean:   []*float32{f32Ptr(1.5), f32Ptr(3)},
			wantMedian: []*float32{f32Ptr(1.5), f32Ptr(3)},
			wantStd:    []*float32{&sampleStd05, f32Ptr(0)},
			wantN:      []uint32{2, 1},
		},
		{
			// A future feature appends a 4th position that only some samples
			// report. Positions 0-2 keep using every sample while position 3
			// is computed from the samples that carry it.
			name: "appended position uses only samples reporting it",
			costs: [][]*float64{
				{f64Ptr(1), f64Ptr(2), f64Ptr(3)},
				{f64Ptr(1), f64Ptr(2), f64Ptr(3)},
				{f64Ptr(1), f64Ptr(2), f64Ptr(3), f64Ptr(10)},
				{f64Ptr(1), f64Ptr(2), f64Ptr(3), f64Ptr(20)},
			},
			wantMean:   []*float32{f32Ptr(1), f32Ptr(2), f32Ptr(3), f32Ptr(15)},
			wantMedian: []*float32{f32Ptr(1), f32Ptr(2), f32Ptr(3), f32Ptr(15)},
			wantStd:    []*float32{f32Ptr(0), f32Ptr(0), f32Ptr(0), &sampleStd50},
			wantN:      []uint32{4, 4, 4, 2},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mean, median, std, gotN := computeVectorStats(c.costs)
			if len(mean) != len(c.wantMean) || len(median) != len(c.wantMedian) ||
				len(std) != len(c.wantStd) || len(gotN) != len(c.wantN) {
				t.Fatalf("lengths: mean=%d median=%d std=%d n=%d; want %d/%d/%d/%d",
					len(mean), len(median), len(std), len(gotN),
					len(c.wantMean), len(c.wantMedian), len(c.wantStd), len(c.wantN))
			}
			for i := range mean {
				checkCostPtr(t, "mean", mean[i], c.wantMean[i])
				checkCostPtr(t, "median", median[i], c.wantMedian[i])
				checkCostPtr(t, "std", std[i], c.wantStd[i])
				if gotN[i] != c.wantN[i] {
					t.Fatalf("n[%d] = %d, want %d", i, gotN[i], c.wantN[i])
				}
			}
		})
	}
}

func TestProcessDataFillsStats(t *testing.T) {
	now := time.Now()
	record := NumericalTaskRecord{
		ScoresSamples: []ScoresSample{
			{Score: 1, RunTime: 1, StatusCode: 0, Cost: []*float64{f64Ptr(1), f64Ptr(2), nil}},
			{Score: 2, RunTime: 2, StatusCode: 0, Cost: []*float64{f64Ptr(4), f64Ptr(5), f64Ptr(6)}},
			{Score: 3, RunTime: 3, StatusCode: 0, Cost: []*float64{f64Ptr(7), f64Ptr(8), f64Ptr(9)}},
		},
		CircBuffer: types.CircularBuffer{
			CircBufferLen: 3,
			NumSamples:    3,
			Times:         []time.Time{now, now, now},
			Indexes:       types.CircularIndexes{Start: 0, End: 2},
		},
	}

	l := zerolog.Nop()
	if err := record.ProcessData(&l); err != nil {
		t.Fatalf("ProcessData: %v", err)
	}

	// Scalar stats get the sample count.
	if record.Stats.Score.N != 3 {
		t.Fatalf("Stats.Score.N = %d, want 3", record.Stats.Score.N)
	}
	if record.Stats.Time.N != 3 {
		t.Fatalf("Stats.Time.N = %d, want 3", record.Stats.Time.N)
	}
	checkCostPtr(t, "score.mean", f32Ptr(record.Stats.Score.Mean), f32Ptr(2))

	// Vector stats.
	if len(record.Stats.Cost.Mean) != 3 {
		t.Fatalf("Stats.Cost.Mean length = %d, want 3", len(record.Stats.Cost.Mean))
	}
	checkCostPtr(t, "cost.mean[0]", record.Stats.Cost.Mean[0], f32Ptr(4))
	checkCostPtr(t, "cost.mean[2]", record.Stats.Cost.Mean[2], f32Ptr(7.5))
	checkCostPtr(t, "cost.median[0]", record.Stats.Cost.Median[0], f32Ptr(4))
	checkCostPtr(t, "cost.median[2]", record.Stats.Cost.Median[2], f32Ptr(7.5))
	checkCostPtr(t, "cost.std[2]", record.Stats.Cost.Std[2], f32Ptr(float32(math.Sqrt(4.5))))

	wantN := []uint32{3, 3, 2}
	for i, want := range wantN {
		if record.Stats.Cost.N[i] != want {
			t.Fatalf("Stats.Cost.N[%d] = %d, want %d", i, record.Stats.Cost.N[i], want)
		}
	}
}

func TestScoresSampleCostBsonRoundTrip(t *testing.T) {
	source := ScoresSample{
		Score:       1.5,
		ID:          7,
		RunTime:     12.5,
		StatusCode:  0,
		ErrorString: "",
		Cost:        []*float64{f64Ptr(10), nil, f64Ptr(30)},
	}

	raw, err := bson.Marshal(source)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ScoresSample
	if err := bson.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.Cost) != 3 {
		t.Fatalf("Cost length = %d, want 3", len(decoded.Cost))
	}
	if decoded.Cost[0] == nil || *decoded.Cost[0] != 10 {
		t.Fatalf("Cost[0] = %v, want 10", decoded.Cost[0])
	}
	if decoded.Cost[1] != nil {
		t.Fatalf("Cost[1] = %v, want nil", *decoded.Cost[1])
	}
	if decoded.Cost[2] == nil || *decoded.Cost[2] != 30 {
		t.Fatalf("Cost[2] = %v, want 30", decoded.Cost[2])
	}
}

func TestNumericalTaskRecordStatsBsonRoundTrip(t *testing.T) {
	source := NumericalTaskRecord{
		Stats: NumericalStatsRecord{
			Score: ScalarStatsRecord{Mean: 0.5, Median: 0.4, Std: 0.1, N: 10},
			Time:  ScalarStatsRecord{Mean: 12, Median: 11, Std: 2, N: 10},
			Cost: VectorStatsRecord{
				Mean:   []*float32{f32Ptr(4), f32Ptr(5), nil},
				Median: []*float32{f32Ptr(4), f32Ptr(5), nil},
				Std:    []*float32{f32Ptr(3), f32Ptr(3), nil},
				N:      []uint32{3, 3, 0},
			},
		},
	}

	raw, err := bson.Marshal(source)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded NumericalTaskRecord
	if err := bson.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Stats.Score.N != 10 || decoded.Stats.Time.N != 10 {
		t.Fatalf("scalar n = %d/%d, want 10/10", decoded.Stats.Score.N, decoded.Stats.Time.N)
	}
	if len(decoded.Stats.Cost.Mean) != 3 || len(decoded.Stats.Cost.N) != 3 {
		t.Fatalf("cost lengths = %d/%d, want 3/3",
			len(decoded.Stats.Cost.Mean), len(decoded.Stats.Cost.N))
	}
	if decoded.Stats.Cost.Mean[0] == nil || *decoded.Stats.Cost.Mean[0] != 4 {
		t.Fatalf("cost mean[0] = %v, want 4", decoded.Stats.Cost.Mean[0])
	}
	if decoded.Stats.Cost.Mean[2] != nil {
		t.Fatalf("cost mean[2] = %v, want nil", *decoded.Stats.Cost.Mean[2])
	}
	if decoded.Stats.Cost.N[2] != 0 {
		t.Fatalf("cost n[2] = %d, want 0", decoded.Stats.Cost.N[2])
	}
}

// Sanity check that inserting a sample copies the cost vector into the buffer.
func TestInsertSampleCopiesCost(t *testing.T) {
	l := zerolog.Nop()
	record := NumericalTaskRecord{}
	record.NewTask(primitive.NewObjectID(), "lmeh", "task", types.EpochStart.UTC(), &l)

	sample := ScoresSample{
		Score:      1,
		RunTime:    2,
		StatusCode: 0,
		Cost:       []*float64{f64Ptr(11), f64Ptr(22), nil},
	}
	if _, err := record.InsertSample(time.Now(), sample, &l); err != nil {
		t.Fatalf("InsertSample: %v", err)
	}

	got := record.ScoresSamples[record.CircBuffer.Indexes.End].Cost
	if len(got) != 3 {
		t.Fatalf("buffered cost length = %d, want 3", len(got))
	}
	if got[0] == nil || *got[0] != 11 {
		t.Fatalf("buffered cost[0] = %v, want 11", got[0])
	}
	if got[2] != nil {
		t.Fatalf("buffered cost[2] = %v, want nil", *got[2])
	}
}
