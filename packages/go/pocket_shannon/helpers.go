package pocket_shannon

import (
	"packages/pocket_shannon/types"

	apptypes "github.com/pokt-network/poktroll/x/application/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sdk "github.com/pokt-network/shannon-sdk"
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
func EndpointsFromSession(session sessiontypes.Session) (map[types.EndpointAddr]Endpoint, error) {
	sf := sdk.SessionFilter{
		Session: &session,
	}

	allEndpoints, err := sf.AllEndpoints()
	if err != nil {
		return nil, err
	}

	endpoints := make(map[types.EndpointAddr]Endpoint)
	for _, supplierEndpoints := range allEndpoints {
		for _, supplierEndpoint := range supplierEndpoints {
			endpoint := Endpoint{
				Supplier: string(supplierEndpoint.Supplier()),
				Url:      supplierEndpoint.Endpoint().Url,
				// Set the session field on the endpoint for efficient lookup when sending relays.
				Session: session,
			}
			endpoints[endpoint.Addr()] = endpoint
		}
	}

	return endpoints, nil
}

// For a given App session, returns all suppliers associated with it
func SupliersInAppSession(session sessiontypes.Session) ([]sdk.SupplierAddress, error) {
	sf := sdk.SessionFilter{
		Session: &session,
	}

	allEndpoints, err := sf.AllEndpoints()
	if err != nil {
		return nil, err
	}

	out := make([]sdk.SupplierAddress, 0, len(allEndpoints)) // Initialize a slice with capacity for efficiency
	for supAddress := range allEndpoints {
		out = append(out, supAddress)
	}
	return out, nil
}

// For a list of apps, return all the supplier addresses that are in session on each of the services, without repeating
// TODO : It would be nice to change all this into a SDK version of `pocketd query supplier list-suppliers --chain-id <chainID>`
func SupliersInSession(FullNode *LazyFullNode, Apps []string, ServiceIDs []string) (map[string][]sdk.SupplierAddress, error) {

	supplierSeen := make(map[string]map[sdk.SupplierAddress]bool)
	uniqueSuppliers := make(map[string][]sdk.SupplierAddress, 0)

	// For all Apps
	for _, thisApp := range Apps {
		// For all services
		for _, thisService := range ServiceIDs {

			// Get App session
			appSession, err := FullNode.GetSession(types.ServiceID(thisService), thisApp)
			if err != nil {
				return nil, err
			}

			// Get all suppliers here
			appSupliers, err := SupliersInAppSession(appSession)
			if err != nil {
				return nil, err
			}

			// Add to list of unique
			for _, thisSupplier := range appSupliers {
				if !supplierSeen[thisService][thisSupplier] {
					supplierSeen[thisService][thisSupplier] = true
					uniqueSuppliers[thisService] = append(uniqueSuppliers[thisService], thisSupplier)
				}
			}

		}
	}

	return uniqueSuppliers, nil

}
