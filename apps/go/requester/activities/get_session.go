package activities

import (
	"context"
	"errors"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	poktGoSigner "github.com/pokt-foundation/pocket-go/signer"
	poktGoUtils "github.com/pokt-foundation/pocket-go/utils"
	"go.temporal.io/sdk/temporal"
	poktRpc "packages/pocket_rpc"
	poktRpcCommon "packages/pocket_rpc/common"
)

type GetSessionParams struct {
	App     string `json:"app"`
	Service string `json:"service"`
}

var GetSessionName = "get_session"

func (aCtx *Ctx) GetSession(_ context.Context, params GetSessionParams) (*poktGoSdk.DispatchOutput, error) {
	if ok := poktGoUtils.ValidateAddress(params.App); !ok {
		return nil, temporal.NewNonRetryableApplicationError("bad params", "BadParams", nil)
	}

	if e := poktRpcCommon.ServiceIdentifierVerification(params.Service); e != nil {
		return nil, temporal.NewNonRetryableApplicationError("bad params", "BadParams", e)
	}

	var signer *poktGoSigner.Signer
	var ok bool
	if signer, ok = aCtx.App.SignerByAddress.Load(params.App); !ok {
		return nil, temporal.NewApplicationError("application not found", "ApplicationNotFound", nil)
	}

	result, err := aCtx.App.PocketRpc.GetSession(signer.GetPublicKey(), params.Service)
	if err != nil {
		if errors.Is(err, poktRpc.ErrBadRequestParams) {
			return nil, temporal.NewNonRetryableApplicationError("bad params", "BadParams", err)
		}
		return nil, temporal.NewApplicationError("unable to get session", "GetSession", err)
	}

	return result, nil
}
