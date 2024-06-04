package types

import (
	"errors"
	"time"

	"github.com/rs/zerolog"
)

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
func (buffer *CircularBuffer) StepIndex(step int, marker string) error {
	// Get values
	var currValue uint32
	var limitValue uint32
	if marker == "start" {
		currValue = buffer.Indexes.Start
		limitValue = buffer.Indexes.End
	} else if marker == "end" {
		currValue = buffer.Indexes.End
		limitValue = buffer.Indexes.Start
	} else {
		return errors.New("buffer: invalid marker designation")
	}

	// perform the step
	nextVal := int(currValue) + step

	// Check limits and assign value
	currValue = buffer.BufferLimitCheck(nextVal, limitValue)

	// Update values
	if marker == "start" {
		buffer.Indexes.Start = currValue
	} else {
		buffer.Indexes.End = currValue
	}
	buffer.NumSamples = uint32(int(buffer.NumSamples) + step)

	return nil
}

func (buffer *CircularBuffer) CycleIndexes(sampleTTLDays uint32, l *zerolog.Logger) error {

	// Maximum age of a sample
	maxAge := time.Duration(sampleTTLDays) * 24 * time.Hour
	// Check the date of the index start
	oldestAge := time.Since(buffer.Times[buffer.Indexes.Start])

	for oldestAge >= maxAge {
		// Increment the start
		err := buffer.StepIndex(1, "start")
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

func (buffer *CircularBuffer) BufferLimitCheck(nextVal int, limitValue uint32) uint32 {
	// Check for overflow
	if nextVal >= int(buffer.CircBufferLen) {
		nextVal = 0
	} else if nextVal <= 0 {
		// Check for underflow
		nextVal = int(buffer.CircBufferLen - 1)
	}

	// Check for limit
	if nextVal >= int(limitValue) {
		nextVal = int(limitValue)
	}

	return uint32(nextVal)
}
