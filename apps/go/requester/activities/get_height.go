package activities

import (
	"context"
)

type GetHeightResults struct {
	Height int64 `json:"height"`
}

var GetHeightName = "get_height"

func (aCtx *Ctx) GetHeight(ctx context.Context) (*GetHeightResults, error) {
	result := GetHeightResults{}
	return &result, nil
}
