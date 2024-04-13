package pocket_rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"io"
	"net/http"
	"time"
)

// todo: cleanup this list after we get everything we really need.
// this was a copy paste from the pokt go package but they expose as V1Route
const (
	// ClientChallengeRoute represents client challenge route
	ClientChallengeRoute = "/v1/client/challenge"
	// ClientDispatchRoute represents client dispatch route
	ClientDispatchRoute = "/v1/client/dispatch"
	// ClientRawTXRoute represents cliente raw TX route
	ClientRawTXRoute = "/v1/client/rawtx"
	// ClientRelayRoute represents client realy route
	ClientRelayRoute = "/v1/client/relay"
	// QueryAccountRoute represents query account route
	QueryAccountRoute = "/v1/query/account"
	// QueryAccountsRoute represents query accounts route
	QueryAccountsRoute = "/v1/query/accounts"
	// QueryAccountTXsRoute represents query account TXs route
	QueryAccountTXsRoute = "/v1/query/accounttxs"
	// QueryAllParamsRoute represents query all params route
	QueryAllParamsRoute = "/v1/query/allparams"
	// QueryAppRoute represents query app route
	QueryAppRoute = "/v1/query/app"
	// QueryAppParamsRoute represents query app params route
	QueryAppParamsRoute = "/v1/query/appparams"
	// QueryAppsRoute represents query apps route
	QueryAppsRoute = "/v1/query/apps"
	// QueryBalanceRoute represents query balance route
	QueryBalanceRoute = "/v1/query/balance"
	// QueryBlockRoute represents query block route
	QueryBlockRoute = "/v1/query/block"
	// QueryBlockTXsRoute represents query block TXs route
	QueryBlockTXsRoute = "/v1/query/blocktxs"
	// QueryHeightRoute represents query height route
	QueryHeightRoute = "/v1/query/height"
	// QueryNodeRoute represents query node route
	QueryNodeRoute = "/v1/query/node"
	// QueryNodeClaimRoute represents query node claim route
	QueryNodeClaimRoute = "/v1/query/nodeclaim"
	// QueryNodeClaimsRoute represents query node claims route
	QueryNodeClaimsRoute = "/v1/query/nodeclaims"
	// QueryNodeParamsRoute represents query node params route
	QueryNodeParamsRoute = "/v1/query/nodeparams"
	// QueryNodeReceiptRoute represents query node receipt route
	QueryNodeReceiptRoute = "/v1/query/nodereceipt"
	// QueryNodeReceiptsRoute represents query node receipts route
	QueryNodeReceiptsRoute = "/v1/query/nodereceipts"
	// QueryNodesRoute represents query nodes route
	QueryNodesRoute = "/v1/query/nodes"
	// QueryPocketParamsRoute represents query pocket params route
	QueryPocketParamsRoute = "/v1/query/pocketparams"
	// QuerySupplyRoute represents query supply route
	QuerySupplyRoute = "/v1/query/supply"
	// QuerySupportedChainsRoute represents query supported chains route
	QuerySupportedChainsRoute = "/v1/query/supportedchains"
	// QueryTXRoute represents query TX route
	QueryTXRoute = "/v1/query/tx"
	// QueryUpgradeRoute represents query upgrade route
	QueryUpgradeRoute = "/v1/query/upgrade"
)

var (
	ErrOnRelayRequest          = errors.New("error on relay request")
	ErrMarshalingRequestParams = errors.New("error marshaling request params")
	ErrCreatingRequest         = errors.New("error creating request")
)

type Rpc interface {
	GetClientPool() *ClientPool
	SetClientPool(clientPool *ClientPool)
	GetApp(address string) (*poktGoSdk.App, error)
}

type PocketRpc struct {
	clientPool *ClientPool
}

func NewPocketRpc(clientPool *ClientPool) *PocketRpc {
	pocketRpc := PocketRpc{}
	pocketRpc.SetClientPool(clientPool)
	return &pocketRpc
}

func readResponse[T interface{}](resp *http.Response) (*T, error) {
	if resp.StatusCode == http.StatusBadRequest {
		return nil, returnRpcError(QueryAppsRoute, resp.Body)
	}

	if string(resp.Status[0]) == "4" {
		return nil, poktGoSdk.Err4xxOnConnection
	}

	if string(resp.Status[0]) == "5" {
		return nil, poktGoSdk.Err5xxOnConnection
	}

	if string(resp.Status[0]) == "2" {
		var r T
		decodeError := json.NewDecoder(resp.Body).Decode(&r)

		if decodeError != nil {
			return nil, poktGoSdk.ErrNonJSONResponse
		}

		return &r, nil
	}

	return nil, poktGoSdk.ErrUnexpectedCodeOnConnection
}

func returnRpcError(route string, body io.ReadCloser) error {
	if route == ClientRelayRoute {
		return ErrOnRelayRequest
	}

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	output := poktGoSdk.RPCError{}

	err = json.Unmarshal(bodyBytes, &output)
	if err != nil {
		return err
	}

	return &output
}

func (rpc *PocketRpc) GetClientPool() *ClientPool {
	return rpc.clientPool
}

func (rpc *PocketRpc) SetClientPool(clientPool *ClientPool) {
	rpc.clientPool = clientPool
}

func (rpc *PocketRpc) GetApp(address string) (*poktGoSdk.App, error) {
	params := map[string]any{
		"height":  0,
		"address": address,
	}

	payloadBytes, err := json.Marshal(params)
	if err != nil {
		rpc.clientPool.logger.Error().Err(err).Msg("error occurred while encoding data")
		return nil, ErrMarshalingRequestParams
	}

	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", QueryAppRoute, body)
	if err != nil {
		rpc.clientPool.logger.Error().Err(err).Msg("error occurred creating http.NewRequest")
		return nil, ErrCreatingRequest
	}

	req.Header.Set("Content-Type", "application/json")

	resp, _, err := rpc.clientPool.DoRRLoadBalanced(req, 10*time.Second)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		closeError := Body.Close()
		if closeError != nil {
			rpc.clientPool.logger.Error().Err(closeError).Msg("error defering body close")
		}
	}(resp.Body)

	return readResponse[poktGoSdk.App](resp)
}
