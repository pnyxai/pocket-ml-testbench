package common

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	poktGoSigner "github.com/pokt-foundation/pocket-go/signer"
	"golang.org/x/crypto/sha3"
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

	unsignedAAt := poktGoSdk.PocketAAT{
		Version:      CurrentAATVersion,
		AppPubKey:    signer.GetPublicKey(),
		ClientPubKey: signer.GetPublicKey(),
		Signature:    "",
	}

	marshaledAAT, err := json.Marshal(unsignedAAt)
	if err != nil {
		return nil, err
	}

	hasher := sha3.New256()

	_, err = hasher.Write(marshaledAAT)
	if err != nil {
		return nil, err
	}

	unsignedAAtHash := hasher.Sum(nil)

	s, e := signer.Sign(unsignedAAtHash)
	if e != nil {
		return nil, e
	}

	signedAAt := unsignedAAt
	signedAAt.Signature = s

	return &signedAAt, nil
}
