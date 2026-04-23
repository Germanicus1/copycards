package config

import "fmt"

// endpointHost is the base host for Flowboards' REST API. The full REST
// URL for any org is deterministic: <host>/rest/2/<orgID>. Callers can
// override this per-org by setting endpoint = "..." in the config file
// (useful for mock servers or alternate hosts).
const endpointHost = "https://n1.flowboards.kanban.plus"

// BuildEndpoint returns the REST API base URL for an org ID.
func BuildEndpoint(orgID string) string {
	return fmt.Sprintf("%s/rest/2/%s", endpointHost, orgID)
}
