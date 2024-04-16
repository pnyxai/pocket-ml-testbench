package pocket_rpc

import (
	"errors"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
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
	GetNodes(service string) ([]*poktGoSdk.Node, error)
}
