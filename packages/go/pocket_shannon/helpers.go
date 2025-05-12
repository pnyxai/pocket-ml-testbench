package pocket_shannon

import (
	"packages/pocket_shannon/types"

	apptypes "github.com/pokt-network/poktroll/x/application/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sdk "github.com/pokt-network/shannon-sdk"
	"github.com/rs/zerolog"
)

func AppIsStakedForService(serviceID types.ServiceID, app *apptypes.Application) bool {
	for _, svcCfg := range app.ServiceConfigs {
		if types.ServiceID(svcCfg.ServiceId) == serviceID {
			return true
		}
	}

	return false
}

// endpointsFromSession returns the list of all endpoints from a Shannon session.
// It returns a map for efficient lookup, as the main/only consumer of this function uses
// the return value for selecting an endpoint for sending a relay.
func EndpointsFromSession(session sessiontypes.Session) (map[string]Endpoint, error) {
	sf := sdk.SessionFilter{
		Session: &session,
	}

	allEndpoints, err := sf.AllEndpoints()
	if err != nil {
		return nil, err
	}

	endpoints := make(map[string]Endpoint)
	for _, supplierEndpoints := range allEndpoints {
		for _, supplierEndpoint := range supplierEndpoints {
			endpoint := Endpoint{
				Supplier: string(supplierEndpoint.Supplier()),
				Url:      supplierEndpoint.Endpoint().Url,
				// Set the session field on the endpoint for efficient lookup when sending relays.
				Session: session,
			}
			endpoints[endpoint.GetSupplier()] = endpoint
		}
	}

	return endpoints, nil
}

// For a given App session, returns all suppliers associated with it
func SupliersInAppSession(session sessiontypes.Session, l *zerolog.Logger) ([]sdk.SupplierAddress, error) {
	sf := sdk.SessionFilter{
		Session: &session,
	}

	allEndpoints, err := sf.AllEndpoints()
	if err != nil {
		l.Debug().Msg("Failed to get endpoint from supplier.")
		return nil, err
	}

	out := make([]sdk.SupplierAddress, 0, len(allEndpoints)) // Initialize a slice with capacity for efficiency
	for supAddress := range allEndpoints {
		out = append(out, supAddress)
	}
	return out, nil
}

// Get the session data and connected nodes for a list of apps and services
func GetAllSessions(FullNode *LazyFullNode, Apps []string, ServiceIDs []string, failOnError bool, l *zerolog.Logger) ([]sessiontypes.Session, error) {

	sessions := make([]sessiontypes.Session, 0)
	// For all Apps
	for _, thisApp := range Apps {
		l.Debug().Str("thisApp", thisApp).Msg("Checking App:")
		// For all services
		for _, thisService := range ServiceIDs {
			l.Debug().Str("thisService", thisService).Msg("Checking Service:")

			// Get App session
			appSession, err := FullNode.GetSession(types.ServiceID(thisService), thisApp)
			if err != nil {
				l.Debug().Msg("Failed to get session.")
				if failOnError {
					return nil, err
				}
			}

			sessions = append(sessions, appSession)
		}
	}
	return sessions, nil
}

// For a list of apps, return all the supplier addresses that are in session on each of the services, without repeating
// TODO : It would be nice to change all this into a SDK version of `pocketd query supplier list-suppliers --chain-id <chainID>`
func SupliersInSession(FullNode *LazyFullNode, Apps []string, ServiceIDs []string, l *zerolog.Logger) (map[string][]sdk.SupplierAddress, error) {

	supplierSeen := make(map[string]map[sdk.SupplierAddress]bool)
	uniqueSuppliers := make(map[string][]sdk.SupplierAddress, 0)

	// Get all sessions
	allSessions, err := GetAllSessions(FullNode, Apps, ServiceIDs, true, l)
	if err != nil {
		return nil, err
	}

	// Process
	for _, appSession := range allSessions {

		// Recover the session ID
		thisService := appSession.Header.ServiceId

		// Get all suppliers here
		appSupliers, err := SupliersInAppSession(appSession, l)
		if err != nil {
			l.Debug().Msg("Failed to get suppliers in session.")
			return nil, err
		}

		// Add to list of unique
		for _, thisSupplier := range appSupliers {
			if _, ok := supplierSeen[thisService]; !ok {
				supplierSeen[thisService] = make(map[sdk.SupplierAddress]bool)
			}
			if !supplierSeen[thisService][thisSupplier] {
				supplierSeen[thisService][thisSupplier] = true
				uniqueSuppliers[thisService] = append(uniqueSuppliers[thisService], thisSupplier)
			} else {
				l.Debug().Str("thisService", thisService).Str("thisSupplier", string(thisSupplier)).Msg("Duplicate supplier found.")
			}
		}

	}

	return uniqueSuppliers, nil

}
