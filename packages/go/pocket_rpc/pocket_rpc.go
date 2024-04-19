package pocket_rpc

import (
	"bytes"
	"context"
	"encoding/json"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"io"
	"net/http"
	"time"
)

type PocketRpc struct {
	clientPool *ClientPool
	pageSize   int
}

type PageAndServiceParams struct {
	// this one on v0 is still called blockchain, but will be service on v1
	Service string `json:"blockchain"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

type HeightAndOptsParams struct {
	Height int64                `json:"height"`
	Opts   PageAndServiceParams `json:"opts"`
}

type NodesPageChannelResponse struct {
	Data  *poktGoSdk.GetNodesOutput
	Error error
}

func NewPocketRpc(clientPool *ClientPool) *PocketRpc {
	pocketRpc := PocketRpc{pageSize: 1000}
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
		b, _ := io.ReadAll(resp.Body)
		decodeError := json.Unmarshal(b, &r)
		//decodeError := json.NewDecoder(resp.Body).Decode(&r)

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

	reqCtx, cancelFunc := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancelFunc()

	resp, _, err := rpc.clientPool.DoRRLoadBalanced(req, reqCtx)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		closeError := Body.Close()
		if closeError != nil {
			rpc.clientPool.logger.Error().Err(closeError).Msg("error deferring body close")
		}
	}(resp.Body)

	return readResponse[poktGoSdk.App](resp)
}

func (rpc *PocketRpc) getNodesByPage(service string, page int, pageSize int, ch chan NodesPageChannelResponse) {
	chResponse := NodesPageChannelResponse{}
	defer func(ch chan<- NodesPageChannelResponse, response *NodesPageChannelResponse) {
		ch <- *response
	}(ch, &chResponse)
	params := HeightAndOptsParams{
		Height: 0,
		Opts: PageAndServiceParams{
			Service: service,
			Page:    page,
			PerPage: pageSize,
		},
	}

	payloadBytes, err := json.Marshal(params)
	if err != nil {
		rpc.clientPool.logger.Error().Err(err).Msg("error occurred while encoding data")
		chResponse.Error = ErrMarshalingRequestParams
		return
	}

	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", QueryNodesRoute, body)
	if err != nil {
		rpc.clientPool.logger.Error().Err(err).Msg("error occurred creating http.NewRequest")
		chResponse.Error = ErrCreatingRequest
		return
	}

	req.Header.Set("Content-Type", "application/json")

	reqCtx, cancelFunc := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancelFunc()

	resp, _, err := rpc.clientPool.DoRRLoadBalanced(req, reqCtx)
	if err != nil {
		chResponse.Error = err
		return
	}

	defer func(Body io.ReadCloser) {
		closeError := Body.Close()
		if closeError != nil {
			rpc.clientPool.logger.Error().Err(closeError).Msg("error deferring body close")
		}
	}(resp.Body)

	chResponse.Data, chResponse.Error = readResponse[poktGoSdk.GetNodesOutput](resp)
}

func (rpc *PocketRpc) GetNodes(service string) (nodes []*poktGoSdk.Node, e error) {
	nodes = make([]*poktGoSdk.Node, 0)
	chGetNodes := make(chan NodesPageChannelResponse, 5)
	defer close(chGetNodes)

	rpc.getNodesByPage(service, 1, rpc.pageSize, chGetNodes)

	firstNodesPage := <-chGetNodes
	if firstNodesPage.Error != nil {
		e = firstNodesPage.Error
		return
	}

	totalPages := firstNodesPage.Data.TotalPages
	chNextPages := make(chan NodesPageChannelResponse, totalPages-1)
	defer close(chNextPages)

	for i := 1; i < totalPages; i++ {
		go rpc.getNodesByPage(service, i+1, rpc.pageSize, chNextPages)
	}

	nodes = append(nodes, firstNodesPage.Data.Result...)

	for i := 0; i < totalPages-1; i++ {
		nodesPage := <-chNextPages
		if nodesPage.Error != nil {
			e = nodesPage.Error
			return
		}
		nodes = append(nodes, nodesPage.Data.Result...)
	}

	return
}

func (rpc *PocketRpc) GetBlock(height int64) (*poktGoSdk.GetBlockOutput, error) {
	params := map[string]any{
		"height": height,
	}

	payloadBytes, err := json.Marshal(params)
	if err != nil {
		rpc.clientPool.logger.Error().Err(err).Msg("error occurred while encoding data")
		return nil, ErrMarshalingRequestParams
	}

	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", QueryBlockRoute, body)
	if err != nil {
		rpc.clientPool.logger.Error().Err(err).Msg("error occurred creating http.NewRequest")
		return nil, ErrCreatingRequest
	}

	req.Header.Set("Content-Type", "application/json")

	reqCtx, cancelFunc := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancelFunc()

	resp, _, err := rpc.clientPool.DoRRLoadBalanced(req, reqCtx)
	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		closeError := Body.Close()
		if closeError != nil {
			rpc.clientPool.logger.Error().Err(closeError).Msg("error deferring body close")
		}
	}(resp.Body)

	r, e := readResponse[poktGoSdk.GetBlockOutput](resp)

	return r, e
}

func (rpc *PocketRpc) GetAllParams(height int64) (*poktGoSdk.AllParams, error) {
	params := map[string]any{
		"height": height,
	}

	payloadBytes, err := json.Marshal(params)
	if err != nil {
		rpc.clientPool.logger.Error().Err(err).Msg("error occurred while encoding data")
		return nil, ErrMarshalingRequestParams
	}

	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", QueryAllParamsRoute, body)
	if err != nil {
		rpc.clientPool.logger.Error().Err(err).Msg("error occurred creating http.NewRequest")
		return nil, ErrCreatingRequest
	}

	req.Header.Set("Content-Type", "application/json")

	reqCtx, cancelFunc := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancelFunc()

	resp, _, err := rpc.clientPool.DoRRLoadBalanced(req, reqCtx)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		closeError := Body.Close()
		if closeError != nil {
			rpc.clientPool.logger.Error().Err(closeError).Msg("error deferring body close")
		}
	}(resp.Body)

	return readResponse[poktGoSdk.AllParams](resp)
}

func (rpc *PocketRpc) GetSession(application, service string) (*poktGoSdk.DispatchOutput, error) {
	params := map[string]any{
		"app_public_key": application,
		"chain":          service,
	}

	payloadBytes, err := json.Marshal(params)
	if err != nil {
		rpc.clientPool.logger.Error().Err(err).Msg("error occurred while encoding data")
		return nil, ErrMarshalingRequestParams
	}

	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", ClientDispatchRoute, body)
	if err != nil {
		rpc.clientPool.logger.Error().Err(err).Msg("error occurred creating http.NewRequest")
		return nil, ErrCreatingRequest
	}

	req.Header.Set("Content-Type", "application/json")

	reqCtx, cancelFunc := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancelFunc()

	resp, _, err := rpc.clientPool.DoRRLoadBalanced(req, reqCtx)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		closeError := Body.Close()
		if closeError != nil {
			rpc.clientPool.logger.Error().Err(closeError).Msg("error deferring body close")
		}
	}(resp.Body)

	return readResponse[poktGoSdk.DispatchOutput](resp)
}
