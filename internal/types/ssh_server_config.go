package types

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// ServerConfig contains cluster-wide SSH parameters
type ServerConfig struct {
	BindAddress string `mapstructure:"bind_address"`
	HostKeyFile string `mapstructure:"host_key_file"`
	DebugImage  string `mapstructure:"debug_image"`
}

// Default configuration values
var serverConfigDefault = []byte(`
bind_address: ":2222"
host_key_file: "utils/config/sample_key"
debug_image: "busybox"
`)

func GetServerConf(configPath string) (*ServerConfig, error) {

	viper.SetConfigType("yaml")
	if err := viper.ReadConfig(bytes.NewBuffer(serverConfigDefault)); err != nil {
		return nil, err
	}

	conf := &ServerConfig{}
	if err := viper.Unmarshal(conf); err != nil {
		return nil, fmt.Errorf("unable to decode into config struct, %v", err)
	}

	// Overlay configuration from file if any
	if configPath != "" {
		file, err := os.Open(configPath)
		if err != nil {
			return nil, fmt.Errorf("can't read config file at %s: %v", configPath, err)
		}
		if err = viper.ReadConfig(file); err != nil {
			return nil, fmt.Errorf("error reading config file at %s: %v", configPath, err)
		}
		if err = viper.Unmarshal(conf); err != nil {
			return nil, fmt.Errorf("unable to decode into config struct, %v", err)
		}
	}

	return conf, nil
}
