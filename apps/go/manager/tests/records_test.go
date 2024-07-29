package tests

import (
	"fmt"
	"manager/records"
	"manager/types"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat"
)

// define a test suite struct
type RecordstUnitTestSuite struct {
	BaseSuite
}

func (s *RecordstUnitTestSuite) Test_NumericalTaskRecord() {
	// Create record
	var record records.NumericalTaskRecord
	// Initialize
	record.NewTask(primitive.NewObjectID(), "framework", "task", types.EpochStart.UTC(), s.app.Logger)

	// Create a vector larger than the buffer length to keep as ground truth
	truthVector := make([]float64, records.NumericalCircularBufferLength*2)
	// Fill the vector with random values
	for i := range truthVector {
		truthVector[i] = float64(rand.Intn(1000)) / 10.0
	}

	// Insert some values and check process results
	numInitialInserts := 8
	for step := 0; step < numInitialInserts; step++ {
		// Create score sample
		var sample records.ScoresSample
		sample.Score = truthVector[step]
		sample.ID = step
		// Insert
		err := record.InsertSample(time.Now(), sample, s.app.Logger)
		if err != nil {
			s.T().Error(err)
			return
		}
		// Process record data
		record.ProcessData(s.app.Logger)
	}

	// Get truth data
	partVector := truthVector[:numInitialInserts]
	meanScore := float32(stat.Mean(partVector, nil))
	stdScore := float32(stat.StdDev(partVector, nil))
	var medianScore float32
	sort.Float64s(partVector)
	if numInitialInserts%2 == 0 {
		medianScore = float32((partVector[numInitialInserts/2-1] + partVector[numInitialInserts/2]) / 2)
	} else {
		medianScore = float32(partVector[numInitialInserts/2])
	}
	if meanScore != record.MeanScore {
		s.T().Error(fmt.Errorf("MeanScore wrong (initial insert):  got = %v, want %v", record.MeanScore, meanScore))
	}
	if stdScore != record.StdScore {
		s.T().Error(fmt.Errorf("StdScore wrong (initial insert):  got = %v, want %v", record.StdScore, stdScore))
	}
	if medianScore != record.MedianScore {
		s.T().Error(fmt.Errorf("MedianScore wrong (initial insert):  got = %v, want %v", record.MedianScore, medianScore))
	}

	// Make an overflow
	for step := 0; step < int(records.NumericalCircularBufferLength); step++ {
		// Create score sample
		var sample records.ScoresSample
		sample.Score = truthVector[numInitialInserts+step]
		sample.ID = step
		// Insert
		err := record.InsertSample(time.Now(), sample, s.app.Logger)
		if err != nil {
			s.T().Error(err)
			return
		}
	}
	// Process record data
	record.ProcessData(s.app.Logger)
	// Get truth data
	partVector = truthVector[numInitialInserts:(numInitialInserts + int(records.NumericalCircularBufferLength))]
	meanScore = float32(stat.Mean(partVector, nil))
	stdScore = float32(stat.StdDev(partVector, nil))
	sort.Float64s(partVector)
	if records.NumericalCircularBufferLength%2 == 0 {
		medianScore = float32((partVector[records.NumericalCircularBufferLength/2-1] + partVector[records.NumericalCircularBufferLength/2]) / 2)
	} else {
		medianScore = float32(partVector[records.NumericalCircularBufferLength/2])
	}
	if meanScore != record.MeanScore {
		s.T().Error(fmt.Errorf("MeanScore wrong (overflow insert):  got = %v, want %v", record.MeanScore, meanScore))
	}
	if stdScore != record.StdScore {
		s.T().Error(fmt.Errorf("StdScore wrong (overflow insert):  got = %v, want %v", record.StdScore, stdScore))
	}
	if medianScore != record.MedianScore {
		s.T().Error(fmt.Errorf("MedianScore wrong (overflow insert):  got = %v, want %v", record.MedianScore, medianScore))
	}

}

func (s *RecordstUnitTestSuite) Test_SignatureTaskRecord() {
	// Create record
	var record records.SignatureTaskRecord
	// Initialize
	record.NewTask(primitive.NewObjectID(), "framework", "task", types.EpochStart.UTC(), s.app.Logger)

	// Define the character set
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	// Define the length of the random string
	length := 10
	// Create some signatures
	signatures := []string{}
	numSignaturesTest := 5
	for step := 0; step < numSignaturesTest; step++ {
		// Create the random string
		randString := ""
		for i := 0; i < length; i++ {
			randIndex := rand.Intn(len(chars))
			randString += string(chars[randIndex])
		}
		signatures = append(signatures, randString)
	}

	for sign := range signatures {
		// Create score sample
		var sample records.SignatureSample
		sample.Signature = signatures[sign]
		sample.ID = sign
		// Insert
		err := record.InsertSample(time.Now(), sample, s.app.Logger)
		if err != nil {
			s.T().Error(err)
			return
		}
		// Process record data
		record.ProcessData(s.app.Logger)
	}

	if signatures[numSignaturesTest-1] != record.LastSignature {
		s.T().Error(fmt.Errorf("Signature does not match (initial insert):  got = %v, want %v", record.LastSignature, signatures[numSignaturesTest-1]))
	}

}
