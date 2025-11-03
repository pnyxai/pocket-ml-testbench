package activities

import (
	"context"
	"math/rand/v2"
)

var GetRandomSeedName = "get_random_seed"

func (aCtx *Ctx) GetRandomSeed(ctx context.Context) (int, error) {
	// Just get a random number to be used as a "deterministic randomness".
	// This will be used in all tasks triggered by this workflow, when it is
	// important to keep coordinated randomness (like in signatures)
	return rand.IntN(42424242), nil
}
