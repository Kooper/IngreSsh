package server

import (
	"errors"
	"sync"

	log "github.com/sirupsen/logrus"

	"kuberstein.io/ingressh/internal/types"
)

// RoutingTable maps authorized keys to the relevant configurations for the
// fast search.
type RoutingTable struct {
	configs []types.SshConfig
	routes  map[string][]*types.SshConfig
	mutex   sync.RWMutex
}

var Routes = RoutingTable{
	configs: make([]types.SshConfig, 0),
	routes:  make(map[string][]*types.SshConfig),
}

// Set sets routes for the specified config
//
// First, find the previous configuration (by the Name and Namespace)
// Then compare the previous and the new configuration, finding changes in
// the set of authorizedKeys (to add/delete)
// Then update config object, remove delete set, add add set
// Update set is not needed as the configuration object will be updated inplace
func (r *RoutingTable) Set(newConfig *types.SshConfig) {

	r.mutex.Lock()
	defer r.mutex.Unlock()

	var existingConfig *types.SshConfig
	for i, c := range r.configs {
		if c.Name == newConfig.Name && c.Namespace == newConfig.Namespace {
			existingConfig = &r.configs[i]
			break
		}
	}

	// As with the new config we will add all the keys and remove none of the
	// keys
	addAuthorizedKeys := make(map[string]bool)
	removeAuthorizedKeys := make(map[string]bool)
	for _, a := range newConfig.AuthorizedKeys {
		addAuthorizedKeys[a] = true
	}

	// If it is not a new config - only new keys should be added and old ones
	// not presented in the new configuration should be removed
	if existingConfig != nil {
		for _, a := range existingConfig.AuthorizedKeys {
			_, ok := addAuthorizedKeys[a]
			if ok {
				// The key is in the new and in the old configuration, skip it
				delete(addAuthorizedKeys, a)
			} else {
				// The key is not in the new configuraiton, put it for removal
				removeAuthorizedKeys[a] = true
			}
		}
	}

	// Switch configuration
	if existingConfig != nil {
		*existingConfig = *newConfig
	} else {
		r.configs = append(r.configs, *newConfig)
		existingConfig = &r.configs[len(r.configs)-1]
	}

	for a := range addAuthorizedKeys {
		_, ok := r.routes[a]
		if !ok {
			r.routes[a] = []*types.SshConfig{existingConfig}
		} else {
			r.routes[a] = append(r.routes[a], existingConfig)
		}
	}

	for a := range removeAuthorizedKeys {
		r.deleteAuthorizedKey(a, *existingConfig)
	}
}

// Get returns routes configurations for the specified authorizedKey
// Returns error if no such authorizedKey exists.
func (r *RoutingTable) Get(authorizedKey string) ([]*types.SshConfig, error) {

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	existing, ok := r.routes[authorizedKey]
	if !ok {
		log.Errorf("No user with the authorized key: %s", authorizedKey)
		return nil, errors.New("authentication failure")
	}

	return existing, nil
}

// Delete deletes the specified config.
func (r *RoutingTable) Delete(config *types.SshConfig) {

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Delete the affected routes
	for _, key := range config.AuthorizedKeys {
		r.deleteAuthorizedKey(key, *config)
	}

	// Delete the configuration
	for idx, c := range r.configs {
		if c.Name == config.Name && c.Namespace == config.Namespace {
			r.configs = append(r.configs[:idx], r.configs[idx+1:]...)
		}
	}
}

// deleteAuthorizedKey deletes routes for the specified authorizedKey.
func (r *RoutingTable) deleteAuthorizedKey(authorizedKey string, config types.SshConfig) {
	configs, ok := r.routes[authorizedKey]
	if !ok {
		return
	}
	for idx, c := range configs {
		if c.Name == config.Name && c.Namespace == config.Namespace {
			r.routes[authorizedKey] = append(r.routes[authorizedKey][:idx], r.routes[authorizedKey][idx+1:]...)
			break
		}
	}
	if len(r.routes[authorizedKey]) == 0 {
		delete(r.routes, authorizedKey)
	}
}
