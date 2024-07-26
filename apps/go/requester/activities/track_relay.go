package activities

import (
	"context"
	"packages/logger"
	"requester/types"
)

type TrackRelayParams struct {
	Servicer      string `json:"servicer"`
	Application   string `json:"application"`
	Service       string `json:"service"`
	SessionHeight int64  `json:"session_height"`
	WasError      bool   `json:"was_error"`
	ResponseMs    int64  `json:"response_ms"`
}

var TrackRelayName = "track_relay"

func (aCtx *Ctx) TrackRelay(ctx context.Context, params TrackRelayParams) error {
	l := logger.GetActivityLogger(RelayerName, ctx, nil)
	relayBySession := types.RelaysBySession{
		Servicer:      params.Servicer,
		Application:   params.Application,
		Service:       params.Service,
		SessionHeight: params.SessionHeight,
		IsError:       params.WasError,
		Ms:            params.ResponseMs,
	}
	if err := relayBySession.IncreaseRelay(ctx, aCtx.App.Mongodb.GetCollection(types.RelaysBySessionCollection)); err != nil {
		// NOTE: should we fail the activity or just log the error?
		l.Error("Error updating relays by session", "error", err, "record", relayBySession)
	}
	return nil
}
