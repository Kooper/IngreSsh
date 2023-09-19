package types

import (
	ing "kuberstein.io/ingressh/api/v1"
)

// SshConfig configures an individual SSH route (or host if you
// like), embedding enough information for the server to join authorization
// rules with the target environment selection.
type SshConfig struct {
	ing.IngreSshSpec
	Name      string
	Namespace string
}

// ApplyDefaults adds default values taken from the server configuration
// for the fields having no values.
func (c *SshConfig) ApplyDefaults(serverConfig ServerConfig) {
	if c.Image == "" {
		c.Image = serverConfig.DebugImage
	}
}
