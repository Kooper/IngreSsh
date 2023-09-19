package server

import (
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	log "github.com/sirupsen/logrus"
	gossh "golang.org/x/crypto/ssh"

	"kuberstein.io/ingressh/internal/types"
)

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

var ctxKeySshConfigs = &contextKey{"ssh_configs"}

func PublicKeyAuthHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	authorized_key := strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key)))

	log.Errorf("Authorizing the key: %s", authorized_key)
	ssh_configs, err := Routes.Get(string(authorized_key))
	if err != nil {
		log.Errorf("Public key auth failed for %v: %v", ctx.User(), err)

		// Hold on upon incorrect authentication attempts to prevent
		// brute-forcing of the secrets
		time.Sleep(1 * time.Second)

		return false
	}
	if len(ssh_configs) == 0 {
		log.Errorf("Empty set of SSH routes for %v", ctx.User())
		return false
	}

	ctx.SetValue(ctxKeySshConfigs, ssh_configs)
	return true
}

func GetSshConfigsFromCtx(ctx ssh.Context) []*types.SshConfig {
	return ctx.Value(ctxKeySshConfigs).([]*types.SshConfig)
}
