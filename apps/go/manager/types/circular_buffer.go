package types

import (
	"errors"
	"math"
	"time"

	"github.com/rs/zerolog"
)

// A date used to mark a position in the buffer that was never used
var EpochStart = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)

// Keep track of circular buffer start and end indexes
type CircularIndexes struct {
	Start uint32 `bson:"cir_start"`
	End   uint32 `bson:"cir_end"`
}

type CircularBuffer struct {
	CircBufferLen uint32          `bson:"buffer_len"`
	NumSamples    uint32          `bson:"num_samples"`
	Times         []time.Time     `bson:"times"`
	Indexes       CircularIndexes `bson:"indexes"`
}

// Gets the sample index given a step direction (positive: 1 or negative: -1) and for a given marker (start or end of buffer)
func (buffer *CircularBuffer) StepIndex(step uint32, marker string, positive_step bool, l *zerolog.Logger) error {

	l.Debug().Int("buffer.Indexes.End", int(buffer.Indexes.Start)).Int("buffer.Indexes.End", int(buffer.Indexes.End)).Msg("Circular indexes.")

	// Get values
	var currValue uint32
	if marker == "start" {
		currValue = buffer.Indexes.Start
	} else if marker == "end" {
		currValue = buffer.Indexes.End
	} else {
		return errors.New("buffer: invalid marker designation")
	}

	// perform the step
	var nextVal uint32 = 0
	if positive_step {
		nextVal = currValue + step
	} else {
		nextVal = currValue - step
	}

	// Check limits and assign value
	currValue, err := buffer.BufferLimitCheck(nextVal, l)
	if err != nil {
		return err
	}

	// Update values
	if marker == "start" {
		buffer.Indexes.Start = currValue
	} else {
		if (buffer.Indexes.Start == currValue) && (step > 0) {
			// This means that the end of the buffer advanced into the start of
			// the buffer, we must movethe buffer one position
			buffer.StepIndex(1, "start", true, l)
		}
		buffer.Indexes.End = currValue
	}

	// Calculate number of valid samples
	validIdx, err := buffer.GetBufferValidIndexes(l)
	if err != nil {
		return err
	}
	buffer.NumSamples = uint32(len(validIdx))

	return nil
}

func (buffer *CircularBuffer) CycleIndexes(sampleTTLDays uint32, l *zerolog.Logger) error {

	// Maximum age of a sample
	maxAge := time.Duration(sampleTTLDays) * 24 * time.Hour
	// Check the date of the index start
	oldestAge := time.Since(buffer.Times[buffer.Indexes.Start])

	for oldestAge >= maxAge {
		// Increment the start
		err := buffer.StepIndex(1, "start", true, l)
		if err != nil {
			return err
		}
		// Update the date
		oldestAge = time.Since(buffer.Times[buffer.Indexes.Start])
		// Break if met the limit
		if buffer.Indexes.Start == buffer.Indexes.End {
			l.Info().Msg("Circular buffer collapsed.")
			break
		}
	}

	return nil
}

func (buffer *CircularBuffer) BufferLimitCheck(nextVal uint32, l *zerolog.Logger) (uint32, error) {
	// Check for overflow
	if nextVal >= buffer.CircBufferLen {
		nextVal = 0
	} else if nextVal == math.MaxInt32 {
		// Check for underflow
		nextVal = buffer.CircBufferLen - 1
	}

	return uint32(nextVal), nil
}

func (buffer *CircularBuffer) GetBufferValidIndexes(l *zerolog.Logger) (auxIdx []uint32, err error) {

	idxNow := buffer.Indexes.Start
	for true {
		// If the sample never written, we should ignore it
		if buffer.Times[idxNow] != EpochStart {
			// Add sample to data array
			auxIdx = append(auxIdx, idxNow)
		}
		// run until we complete the circular buffer
		if idxNow == buffer.Indexes.End {
			break
		}
		// perform the step
		nextVal := idxNow + 1
		// Check limits and assign value
		idxNow, err = buffer.BufferLimitCheck(nextVal, l)
		if err != nil {
			return auxIdx, err
		}
	}
	return auxIdx, err
}
