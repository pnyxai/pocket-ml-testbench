package pocket

import "errors"

// RelayResponseCodesEnum is the vocabulary of outcome codes stored against a
// relay response and against every score sample derived from one.
//
// It lives here, rather than next to either of its users, because it has three
// of them and they do not all read the same source: the requester WRITES these
// codes into MongoDB, the manager READS them back to decide which samples count
// against a supplier, and the Python evaluator writes some of them too. It was
// previously declared twice — once in the requester and once in the manager —
// and the two copies had drifted, which silently broke the manager's reading of
// evaluation errors (see Evaluation below). One declaration makes that class of
// bug unrepresentable.
//
// ⚠️ These numbers are persisted. Never renumber an existing member: old
// documents in MongoDB carry the old value, and apps/python/evaluator hard-codes
// some of them by literal. Only append.
type RelayResponseCodesEnum struct {
	// Ok : the backend answered with a 2xx.
	Ok int
	// Relay : the relay never produced an answer — could not sign, could not
	// reach the supplier, timed out.
	Relay int
	// Supplier : the supplier answered, but wrongly — a 5xx from the backend, or
	// a response that failed signature validation.
	Supplier int
	// OutOfSession : the session the prompt was scheduled for is no longer
	// usable, or could not be fetched.
	OutOfSession int
	// BadParams : the request itself was wrong — bad prompt id, bad session
	// height, or a 4xx from the backend.
	BadParams int
	// PromptNotFound : the prompt was not in MongoDB, or was already done.
	PromptNotFound int
	// DatabaseRead : MongoDB could not be read.
	DatabaseRead int
	// PocketRpc : the full node could not be queried (e.g. for block height).
	PocketRpc int
	// SignerNotFound : no signing key is held for the app the relay is billed to.
	SignerNotFound int
	// SignerError : ring-signing the relay request failed. Ours, not theirs.
	SignerError int
	// AATSignature : reserved. Nothing writes this.
	AATSignature int
	// Evaluation : the evaluator could not score the response.
	//
	// NOT imputable to the supplier — they answered, we failed to score it — so
	// it is not punishable and does not enter a supplier's score buffer.
	//
	// 11 is not arbitrary and must not move: it is written as a literal by
	// apps/python/evaluator (`status_code=11,  # Error at evaluation`), which is
	// the only thing that produces it. The manager's old copy of this enum had
	// it at 12 instead, which is the drift this single declaration exists to
	// prevent.
	Evaluation int
	// MinerSignerError : reserved. Nothing writes or reads this today; it exists
	// because the manager's copy declared it. It sits after Evaluation because
	// putting it before is what shifted Evaluation to 12 in that copy.
	MinerSignerError int
}

var RelayResponseCodes = RelayResponseCodesEnum{
	Ok:               0,
	Relay:            1,
	Supplier:         2,
	OutOfSession:     3,
	BadParams:        4,
	PromptNotFound:   5,
	DatabaseRead:     6,
	PocketRpc:        7,
	SignerNotFound:   8,
	SignerError:      9,
	AATSignature:     10,
	Evaluation:       11,
	MinerSignerError: 12,
}

// ResponseCode maps the outcome of a relay onto the code stored for it.
//
// It takes a plain error so callers do not have to type-assert: anything that is
// not a *RelayError is reported as a relay failure, which is the safe reading —
// we have an error and no evidence about whose fault it was.
func ResponseCode(err error) int {
	if err == nil {
		return RelayResponseCodes.Ok
	}

	var relayErr *RelayError
	if !errors.As(err, &relayErr) {
		return RelayResponseCodes.Relay
	}

	switch relayErr.Stage {
	case StageApp:
		return RelayResponseCodes.SignerNotFound
	case StageSession:
		return RelayResponseCodes.OutOfSession
	case StageEndpoint:
		// The supplier is not in this session, or serves nothing on the
		// transport the service is configured for.
		return RelayResponseCodes.Supplier
	case StageSigning:
		return RelayResponseCodes.SignerError
	case StageSending:
		return RelayResponseCodes.Relay
	case StageValidation:
		// The supplier answered with something that does not verify against its
		// own signature, or that could not be unwrapped.
		return RelayResponseCodes.Supplier
	default:
		return RelayResponseCodes.Relay
	}
}
