package common

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	poktGoSigner "github.com/pokt-foundation/pocket-go/signer"
	poktGoUtils "github.com/pokt-foundation/pocket-go/utils"
)

const (
	// NetworkIdentifierLength copied from pocket-core
	NetworkIdentifierLength = 4
	CurrentAATVersion       = "0.0.1"
)

func ServiceIdentifierVerification(service string) error {
	// decode the address
	decodedString, err := hex.DecodeString(service)
	if err != nil {
		return errors.New("the hex string could not be decoded")
	}
	sLen := len(decodedString)
	// ensure Length isn't 0
	if sLen == 0 {
		return errors.New("the hex provided is empty")
	}
	// ensure Length
	if sLen > NetworkIdentifierLength {
		return errors.New("the merkleHash Length is not valid")
	}

	return nil
}

func NewPocketAATFromPrivKey(privKey string) (*poktGoSdk.PocketAAT, error) {
	signer, err := poktGoSigner.NewSignerFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}
	pubKey := poktGoUtils.PublicKeyFromPrivate(privKey)
	aat := poktGoSdk.PocketAAT{
		Version:      CurrentAATVersion,
		AppPubKey:    pubKey,
		ClientPubKey: pubKey,
		Signature:    "",
	}
	b, err := json.Marshal(aat)
	if err != nil {
		return nil, err
	}
	signature, err := signer.Sign(b)
	if err != nil {
		return nil, err
	}
	aat.Signature = signature
	return &aat, nil
}
