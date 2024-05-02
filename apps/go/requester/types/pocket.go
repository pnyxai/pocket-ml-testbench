package types

import (
	poktGoProvider "github.com/pokt-foundation/pocket-go/provider"
	poktGoSigner "github.com/pokt-foundation/pocket-go/signer"
	poktRpcCommon "packages/pocket_rpc/common"
)

type AppAccount struct {
	Signer    *poktGoSigner.Signer
	SignedAAT *poktGoProvider.PocketAAT
}

func NewAppAccount(privateKey string) (*AppAccount, error) {
	signer, err := poktGoSigner.NewSignerFromPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	aat, aatErr := poktRpcCommon.NewPocketAATFromPrivKey(signer.GetPrivateKey())
	if aatErr != nil {
		return nil, aatErr
	}

	return &AppAccount{signer, aat}, nil
}
