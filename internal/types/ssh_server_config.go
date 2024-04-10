package types

import (
	"os"
)

// ServerConfig contains cluster-wide SSH parameters
type ServerConfig struct {
	BindAddress string
	HostKeyFile string
	DebugImage  string
}

func GetServerConf() *ServerConfig {
	return &ServerConfig{
		BindAddress: getEnv("SSH_BIND_ADDRESS", ":8022"),
		HostKeyFile: getEnv("HOST_KEY_FILE", "/secret/ssh-privatekey"),
		DebugImage: getEnv("DEBUG_IMAGE", "busybox"),
	}
}

func getEnv(key string, defaultVal string) string {
    if value, exists := os.LookupEnv(key); exists {
		return value
    }

    return defaultVal
}
