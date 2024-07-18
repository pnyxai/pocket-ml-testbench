package types

import (
	"errors"
	"fmt"
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

	l.Debug().Int("buffer.Indexes.Start", int(buffer.Indexes.Start)).Int("buffer.Indexes.End", int(buffer.Indexes.End)).Int("step", int(step)).Msg("Circular indexes moving.")

	if step > 1 {
		return fmt.Errorf("Steps of length larger than 1 are not supported.")
	}

	// Check step feasibility
	if marker == "start" {
		if !positive_step {
			// Cannot go back in time
			l.Debug().Msg("Cannot move start index back.")
			return nil
		} else if buffer.NumSamples == 0 && positive_step {
			// Cannot move this index
			l.Debug().Msg("Cannot step over end index.")
			return nil
		}
	} else {
		if buffer.NumSamples == 0 && !positive_step {
			// Cannot move this index
			l.Debug().Msg("Cannot step over start index.")
			return nil
		}
	}

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

	// Check buffer limits
	nextVal, err := buffer.BufferLimitCheck(nextVal, l)
	if err != nil {
		return err
	}

	// Update values
	if marker == "start" {

		if buffer.NumSamples == step && positive_step {
			// Cannot reduce the buffer anymore, just invalidate last sample
			buffer.Times[buffer.Indexes.End] = EpochStart
		} else {
			buffer.Indexes.Start = nextVal
		}

	} else {
		if buffer.Indexes.Start == nextVal && positive_step {
			// This means that the end of the buffer advanced into the start of
			// the buffer, we must move the buffer one position only if we move
			// in the positive direction (otherwise we run into the past)
			if positive_step {
				buffer.StepIndex(1, "start", true, l)

			}
			buffer.Indexes.End = nextVal
		} else if buffer.NumSamples == step && !positive_step {
			// Cannot reduce the buffer anymore, just invalidate last sample
			buffer.Times[buffer.Indexes.Start] = EpochStart
		} else {
			buffer.Indexes.End = nextVal
		}

	}

	// Calculate number of valid samples
	if buffer.Indexes.Start == buffer.Indexes.End {
		if buffer.Times[buffer.Indexes.Start] != EpochStart {
			buffer.NumSamples = 1
		} else {
			buffer.NumSamples = 0
		}
	} else if buffer.Indexes.Start < buffer.Indexes.End {
		buffer.NumSamples = buffer.Indexes.End - buffer.Indexes.Start
		if buffer.Times[buffer.Indexes.Start] != EpochStart {
			buffer.NumSamples += 1
		}
	} else {
		buffer.NumSamples = buffer.CircBufferLen - (buffer.Indexes.Start - buffer.Indexes.End) + 1
		// if buffer.Times[buffer.Indexes.Start] != EpochStart {
		// 	buffer.NumSamples += 1
		// }
	}

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

	if nextVal == math.MaxUint32 {
		// Check for underflow
		nextVal = buffer.CircBufferLen - 1
	} else if nextVal >= buffer.CircBufferLen {
		// Check for overflow
		nextVal = 0
	}

	return nextVal, nil
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
			return nil, err
		}
	}
	return auxIdx, err
}
