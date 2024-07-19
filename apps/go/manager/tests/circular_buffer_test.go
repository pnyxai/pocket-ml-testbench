package tests

import (
	"fmt"
	"manager/types"
	"time"
)

// define a test suite struct
type CircularBuffertUnitTestSuite struct {
	BaseSuite
}

// Size of the buffer to test
const testBufferLen uint32 = 50

func (s *CircularBuffertUnitTestSuite) Test_CircularBuffer() {

	// Create a test circular buffer
	timeArray := make([]time.Time, testBufferLen)
	for i := range timeArray {
		timeArray[i] = types.EpochStart.UTC()
	}
	testCircularBuffer := types.CircularBuffer{
		CircBufferLen: testBufferLen,
		NumSamples:    0,
		Times:         timeArray,
		Indexes: types.CircularIndexes{
			Start: 0,
			End:   0,
		},
	}

	// // Create the data vector that will be governed by the buffer
	// dataVector := make([]float64, testBufferLen)

	// // Create a vector larger than the buffer length to keep as ground truth
	// truthVector := make([]float64, testBufferLen*2)

	// ---- Test buffer with unitary steps
	// Check step function
	stepsMove := int(testBufferLen / 2)
	for step := 0; step < stepsMove; step++ {

		// Increment the end
		err := testCircularBuffer.StepIndex(1, "end", true, s.app.Logger)
		if err != nil {
			s.T().Error(err)
			return
		}
		// Add time
		testCircularBuffer.Times[testCircularBuffer.Indexes.End] = time.Now()

	}
	if uint32(stepsMove) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of elements in the buffer is not equal to the number of steps taken:  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, stepsMove, testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}
	// Check number of valid samples
	validIdx, err := testCircularBuffer.GetBufferValidIndexes(s.app.Logger)
	if err != nil {
		s.T().Error(err)
		return
	}
	if uint32(len(validIdx)) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of valid elements in the buffer is not equal to the number of samples counted:  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, uint32(len(validIdx)), testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}
	// Make an overflow
	stepsMove = int(testBufferLen)
	for step := 0; step < stepsMove; step++ {

		// Increment the end
		err := testCircularBuffer.StepIndex(1, "end", true, s.app.Logger)
		if err != nil {
			s.T().Error(err)
			return
		}
		// Add time
		testCircularBuffer.Times[testCircularBuffer.Indexes.End] = time.Now()
	}
	if uint32(testBufferLen) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of elements in the buffer not equal to the buffer length after an overflow:  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, stepsMove, testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}
	// Check number of valid samples
	validIdx, err = testCircularBuffer.GetBufferValidIndexes(s.app.Logger)
	if err != nil {
		s.T().Error(err)
		return
	}
	if uint32(len(validIdx)) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of valid elements in the buffer is not equal to the number of samples counted (after overflow):  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, uint32(len(validIdx)), testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}

	// Go back all samples
	stepsMove = int(testBufferLen)
	for step := 0; step < stepsMove; step++ {

		// Increment the end
		err := testCircularBuffer.StepIndex(1, "end", false, s.app.Logger)
		if err != nil {
			s.T().Error(err)
			return
		}
		// fmt.Print(testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End, testCircularBuffer.NumSamples, "\n")
	}
	if 0 != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of elements in the buffer is not equal to the number of steps taken (moving end backwards):  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, 0, testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}
	// Check number of valid samples
	validIdx, err = testCircularBuffer.GetBufferValidIndexes(s.app.Logger)
	if err != nil {
		s.T().Error(err)
		return
	}
	if uint32(len(validIdx)) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of valid elements in the buffer is not equal to the number of samples counted (moving end backwards):  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, uint32(len(validIdx)), testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}

	// move end 5 and then start 10
	stepsMove = int(5)
	for step := 0; step < stepsMove; step++ {

		// Increment the end
		err := testCircularBuffer.StepIndex(1, "end", true, s.app.Logger)
		if err != nil {
			s.T().Error(err)
			return
		}
		// Add time
		testCircularBuffer.Times[testCircularBuffer.Indexes.End] = time.Now()
	}
	if 5 != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of elements in the buffer is not equal to the number of steps taken (moving end forward):  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, 5, testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}
	// Check number of valid samples
	validIdx, err = testCircularBuffer.GetBufferValidIndexes(s.app.Logger)
	if err != nil {
		s.T().Error(err)
		return
	}
	if uint32(len(validIdx)) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of valid elements in the buffer is not equal to the number of samples counted (moving end forward):  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, uint32(len(validIdx)), testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}
	stepsMove = int(10)
	for step := 0; step < stepsMove; step++ {

		// Increment the end
		err := testCircularBuffer.StepIndex(1, "start", true, s.app.Logger)
		if err != nil {
			s.T().Error(err)
			return
		}

		// fmt.Print(testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End, testCircularBuffer.NumSamples, "\n")
	}
	if 0 != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of elements in the buffer is not equal to the number of steps taken (moving start forward):  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, 0, testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}
	// Check number of valid samples
	validIdx, err = testCircularBuffer.GetBufferValidIndexes(s.app.Logger)
	if err != nil {
		s.T().Error(err)
		return
	}
	if uint32(len(validIdx)) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of valid elements in the buffer is not equal to the number of samples counted (moving start forward):  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, uint32(len(validIdx)), testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}

	// Check  cycling

	// move end 4
	// fmt.Print(testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End, "\n")
	stepsMove = int(4)
	for step := 0; step < stepsMove; step++ {

		// Increment the end
		err := testCircularBuffer.StepIndex(1, "end", true, s.app.Logger)
		if err != nil {
			s.T().Error(err)
			return
		}
		// Add time
		testCircularBuffer.Times[testCircularBuffer.Indexes.End] = time.Now()
	}
	// Change date of start sample to an old one
	validIdx, err = testCircularBuffer.GetBufferValidIndexes(s.app.Logger)
	testCircularBuffer.Times[validIdx[0]] = types.EpochStart
	// Cycle indexes
	err = testCircularBuffer.CycleIndexes(5, s.app.Logger)
	if err != nil {
		s.T().Error(err)
		return
	}
	// Valid samples must be 4
	if uint32(stepsMove-1) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Index cycling not dropping old sample:  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, stepsMove-1, testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}
	// Check number of valid samples
	validIdx, err = testCircularBuffer.GetBufferValidIndexes(s.app.Logger)
	if err != nil {
		s.T().Error(err)
		return
	}
	if uint32(len(validIdx)) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of valid elements in the buffer is not equal to the number of samples counted (index cycling):  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, uint32(len(validIdx)), testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}

}
