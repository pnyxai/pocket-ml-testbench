package pocket

import (
	"bytes"

	servicetypes "github.com/pokt-network/poktroll/x/service/types"
)

// RelayReceipt is the verifiable record of one relay: who asked, who served it,
// under which session, and the two signatures that make those claims checkable
// by anyone holding the public keys.
//
// WHAT IT IS NOT: the onchain Merkle proof. A relay miner accumulates the
// relays it served into an SMST over the whole session and only submits a claim
// (and then a proof) once the session's claim window opens. None of that exists
// at the moment a relay is answered, so no client can return it. What a client
// CAN produce is this — the signed request/response pair — which is what the
// proof is later built out of, and which is independently verifiable on its own.
type RelayReceipt struct {
	// --- session identity (from the signed session header) ---

	SessionId               string `json:"session_id"`
	SessionStartBlockHeight int64  `json:"session_start_block_height"`
	SessionEndBlockHeight   int64  `json:"session_end_block_height"`
	ApplicationAddress      string `json:"application_address"`
	SupplierOperatorAddress string `json:"supplier_operator_address"`
	ServiceId               string `json:"service_id"`

	// --- signatures ---

	// ApplicationSignature is the ring signature the app put on the REQUEST. It
	// proves the relay was authorised against this app's stake — which is what
	// makes it billable — and is what the miner checks before serving.
	ApplicationSignature []byte `json:"application_signature"`

	// SupplierOperatorSignature is the supplier's signature over the RESPONSE.
	// It is the one that matters for measurement: it binds this specific answer
	// to this specific supplier, so a supplier cannot later disown a bad answer.
	SupplierOperatorSignature []byte `json:"supplier_operator_signature"`

	// --- hashes ---

	// RequestSignableHash is the hash the application signature was made over
	// (the marshaled request with the signature field cleared).
	RequestSignableHash []byte `json:"request_signable_hash"`

	// ResponseSignableHash is the hash the supplier signature was made over.
	ResponseSignableHash []byte `json:"response_signable_hash"`

	// ResponsePayloadHash is the SHA256 of the response payload, as set by the
	// relay miner. It exists so an onchain proof can be checked without the
	// payload, which the miner strips before inserting into the SMST. Empty if
	// the miner did not set it.
	ResponsePayloadHash []byte `json:"response_payload_hash,omitempty"`

	// PayloadPrefix is everything in the signed response payload that comes
	// BEFORE the backend's body — the serialized POKTHTTPResponse's status line,
	// headers and framing.
	//
	// It is what makes ResponsePayloadHash checkable from stored data. The
	// signed payload is `prefix || body`, and only the body is worth keeping
	// (it is the answer being measured), so a stored body alone cannot be
	// re-hashed. Keep this next to it and the check is
	// sha256(PayloadPrefix || body) == ResponsePayloadHash. It is small — a
	// header block, not a payload — which is the whole reason to split it out
	// rather than store the payload twice.
	//
	// Empty when the body is not a suffix of the payload, which should not
	// happen: the guard is there because the split is only meaningful if the
	// serialization really does put the body last, and silently returning a
	// wrong prefix would produce a hash check that fails for no visible reason.
	PayloadPrefix []byte `json:"payload_prefix,omitempty"`

	// RelayHash is the hash of the marshaled Relay{Req, Res} pair, exactly as it
	// came off the wire.
	//
	// ⚠️ Treat this as a local identifier, NOT as the SMST key. The relay miner
	// omits the response payload when it inserts a relay into the tree (that is
	// what ResponsePayloadHash is for), so a hash taken over the pair WITH the
	// payload need not equal the key that ends up onchain. Verify against a
	// miner before relying on it to join to claim data.
	RelayHash []byte `json:"relay_hash"`
}

// receiptFrom assembles a receipt from the wire bytes the probe captured.
//
// body is the unwrapped backend response — the same bytes the caller gets in
// Response.Bytes. It is needed to split the signed payload into prefix and
// body; pass nil and PayloadPrefix is simply left empty.
//
// It returns nil rather than an error: the relay itself succeeded and its
// response is valid — pocket-ap already verified the signature — so a receipt
// that cannot be assembled is missing metadata, not a failed relay.
func (c *Client) receiptFrom(probe *relayProbe, supplierAddress string, body []byte) *RelayReceipt {
	if probe.signedRequestBz == nil || probe.responseBz == nil {
		return nil
	}

	var request servicetypes.RelayRequest
	if err := request.Unmarshal(probe.signedRequestBz); err != nil {
		c.logger.Debug().Err(err).Msg("Could not decode the signed relay request for the receipt.")
		return nil
	}

	var response servicetypes.RelayResponse
	if err := response.Unmarshal(probe.responseBz); err != nil {
		c.logger.Debug().Err(err).Msg("Could not decode the signed relay response for the receipt.")
		return nil
	}

	// Bound to locals because GetMeta returns the metadata by value, and its
	// accessors are declared on the pointer.
	requestMeta := request.GetMeta()
	responseMeta := response.GetMeta()

	receipt := &RelayReceipt{
		SupplierOperatorAddress:   supplierAddress,
		ApplicationSignature:      requestMeta.GetSignature(),
		SupplierOperatorSignature: responseMeta.GetSupplierOperatorSignature(),
		ResponsePayloadHash:       response.GetPayloadHash(),
	}

	// The header is read off the REQUEST: that is the one our own signature
	// covers, so it is the copy we know has not been altered in flight.
	if header := requestMeta.GetSessionHeader(); header != nil {
		receipt.SessionId = header.GetSessionId()
		receipt.SessionStartBlockHeight = header.GetSessionStartBlockHeight()
		receipt.SessionEndBlockHeight = header.GetSessionEndBlockHeight()
		receipt.ApplicationAddress = header.GetApplicationAddress()
		receipt.ServiceId = header.GetServiceId()
	}

	if hash, err := request.GetSignableBytesHash(); err == nil {
		receipt.RequestSignableHash = hash[:]
	}
	if hash, err := response.GetSignableBytesHash(); err == nil {
		receipt.ResponseSignableHash = hash[:]
	}

	relay := servicetypes.Relay{Req: &request, Res: &response}
	if hash, err := relay.GetHash(); err == nil {
		receipt.RelayHash = hash[:]
	}

	// The signed payload is the serialized POKTHTTPResponse, whose last field is
	// the body, so whatever precedes the body is the prefix.
	if payload := response.GetPayload(); len(payload) > 0 && bytes.HasSuffix(payload, body) {
		receipt.PayloadPrefix = payload[:len(payload)-len(body)]
	}

	return receipt
}
