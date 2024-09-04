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

// Test the end pointer moving forward. Basic test.
func (s *CircularBuffertUnitTestSuite) Test_CircularBuffer_Step() {

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

}

// Test the overwflow of the buffer and hence the rolling and pushing of the indexes
func (s *CircularBuffertUnitTestSuite) Test_CircularBuffer_Overflow() {

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

	// Make an overflow
	stepsMove := int(testBufferLen) + int(testBufferLen/2)
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
	validIdx, err := testCircularBuffer.GetBufferValidIndexes(s.app.Logger)
	if err != nil {
		s.T().Error(err)
		return
	}
	if uint32(len(validIdx)) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of valid elements in the buffer is not equal to the number of samples counted (after overflow):  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, uint32(len(validIdx)), testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}

}

// Test the end pointer moving backwards and rolling the indexes backward
func (s *CircularBuffertUnitTestSuite) Test_CircularBuffer_BackStep() {

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

	// Make an overflow
	stepsMove := int(testBufferLen) + int(testBufferLen/2)
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

	// Go back all samples
	stepsMove = int(testBufferLen)
	for step := 0; step < stepsMove; step++ {

		// Increment the end
		err := testCircularBuffer.StepIndex(1, "end", false, s.app.Logger)
		if err != nil {
			s.T().Error(err)
			return
		}
	}
	if 0 != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of elements in the buffer is not equal to the number of steps taken (moving end backwards):  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, 0, testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}
	// Check number of valid samples
	validIdx, err := testCircularBuffer.GetBufferValidIndexes(s.app.Logger)
	if err != nil {
		s.T().Error(err)
		return
	}
	if uint32(len(validIdx)) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Number of valid elements in the buffer is not equal to the number of samples counted (moving end backwards):  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, uint32(len(validIdx)), testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}

}

// Move end and then push start into end to force a collapse and result in zero samples
func (s *CircularBuffertUnitTestSuite) Test_CircularBuffer_BackAndForwardStep() {

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

	// move end 5 and then start 10
	stepsMove := int(5)
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
	validIdx, err := testCircularBuffer.GetBufferValidIndexes(s.app.Logger)
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
}

// Test the cycling of samples, meaning dropping the old ones.
// Here we force a sample to be old and check that it is eliminated.
func (s *CircularBuffertUnitTestSuite) Test_CircularBuffer_Cycling() {

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

	// move end 4
	stepsMove := int(4)
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
	// Cycle indexes (nothing should happen)
	cycled, err := testCircularBuffer.CycleIndexes(5, s.app.Logger)
	if cycled {
		s.T().Error(fmt.Errorf("Index cycling signaling sample drop when all samples are up-to-date"))
	}
	// Change date of start sample to an old one
	validIdx, err := testCircularBuffer.GetBufferValidIndexes(s.app.Logger)
	testCircularBuffer.Times[validIdx[0]] = types.EpochStart
	// Cycle indexes
	cycled, err = testCircularBuffer.CycleIndexes(5, s.app.Logger)
	if err != nil {
		s.T().Error(err)
		return
	}
	// Valid samples must be 4
	if uint32(stepsMove-1) != testCircularBuffer.NumSamples {
		s.T().Error(fmt.Errorf("Index cycling not dropping old sample:  got = %v, want %v (Start Idx: %v - End Idx : %v)", testCircularBuffer.NumSamples, stepsMove-1, testCircularBuffer.Indexes.Start, testCircularBuffer.Indexes.End))
	}
	if !cycled {
		s.T().Error(fmt.Errorf("Index cycling not signaling old sample drop"))
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
