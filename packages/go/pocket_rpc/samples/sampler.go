package samples

import (
	"encoding/json"
	"errors"
	"fmt"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"github.com/rs/zerolog"
	"os"
	"packages/pocket_rpc/types"
	"path"
)

var (
	Height                   = "query_height.json"
	Block                    = "query_block.json"
	App                      = "query_app.json"
	Dispatch                 = "query_dispatch.json"
	AllParams                = "query_allparams.json"
	Nodes                    = "query_nodes.json"
	RelaySuccess             = "client_relay.json"
	RelayError               = "client_relay_error.json"
	RelayEvidenceSealedError = "client_relay_evidence_sealed_error.json"
	RelayNonJson             = "client_relay_non_json.json"
	BasePath                 = "."
)

func SetBasePath(bPath string) {
	BasePath = bPath
}

func GetSampleFromFile[T interface{}](filename string) (*T, error) {
	var res T
	// Read the file
	data, err := os.ReadFile(path.Join(BasePath, filename))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to read file: %v", err))
	}

	// Decode the data
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to decode JSON: %v", err))
	}

	return &res, nil
}

func GetHeightMock(logger *zerolog.Logger) *types.QueryHeightOutput {
	if v, err := GetSampleFromFile[types.QueryHeightOutput](Height); err != nil {
		logger.Fatal().Err(err).Msg("Failed to get height mock")
		return nil
	} else {
		return v
	}
}

func GetBlockMock(logger *zerolog.Logger) *poktGoSdk.GetBlockOutput {
	if v, err := GetSampleFromFile[poktGoSdk.GetBlockOutput](Block); err != nil {
		logger.Fatal().Err(err).Msg("Failed to get block mock")
		return nil
	} else {
		return v
	}
}

func GetAppMock(logger *zerolog.Logger) *poktGoSdk.App {
	if v, err := GetSampleFromFile[poktGoSdk.App](App); err != nil {
		logger.Fatal().Err(err).Msg("Failed to get app mock")
		return nil
	} else {
		return v
	}
}

func GetSessionMock(logger *zerolog.Logger) *poktGoSdk.DispatchOutput {
	if v, err := GetSampleFromFile[poktGoSdk.DispatchOutput](Dispatch); err != nil {
		logger.Fatal().Err(err).Msg("Failed to get dispatch mock")
		return nil
	} else {
		return v
	}
}

func GetAllParamsMock(logger *zerolog.Logger) *poktGoSdk.AllParams {
	if v, err := GetSampleFromFile[poktGoSdk.AllParams](AllParams); err != nil {
		logger.Fatal().Err(err).Msg("Failed to get allparams mock")
		return nil
	} else {
		return v
	}
}

func GetNodesMock(logger *zerolog.Logger) *poktGoSdk.GetNodesOutput {
	if v, err := GetSampleFromFile[poktGoSdk.GetNodesOutput](Nodes); err != nil {
		logger.Fatal().Err(err).Msg("Failed to get nodes mock")
		return nil
	} else {
		return v
	}
}

func GetSuccessRelayOutput(logger *zerolog.Logger) *poktGoSdk.RelayOutput {
	if v, err := GetSampleFromFile[poktGoSdk.RelayOutput](RelaySuccess); err != nil {
		logger.Fatal().Err(err).Msg("Failed to get relay mock")
		return nil
	} else {
		return v
	}
}

func GetErroredRelayOutput(logger *zerolog.Logger) *poktGoSdk.RelayErrorOutput {
	if v, err := GetSampleFromFile[poktGoSdk.RelayErrorOutput](RelayError); err != nil {
		logger.Fatal().Err(err).Msg("Failed to get relay error mock")
		return nil
	} else {
		return v
	}
}

func GetEvidenceSealedRelayOutput(logger *zerolog.Logger) *poktGoSdk.RelayErrorOutput {
	if v, err := GetSampleFromFile[poktGoSdk.RelayErrorOutput](RelayEvidenceSealedError); err != nil {
		logger.Fatal().Err(err).Msg("Failed to get relay error mock")
		return nil
	} else {
		return v
	}
}
