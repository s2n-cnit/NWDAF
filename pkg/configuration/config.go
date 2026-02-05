package configuration

// Description: This file contains the configuration structure and the function to load the configuration from a file.

import (
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/s2n-cnit/nwdaf/pkg/utils"
	"github.com/s2n-cnit/nwdaf/plugin/plugin_shared"
	"gopkg.in/yaml.v3"
)

const (
	// ConfigFile is the default configuration file
	ConfigFile = "config/config.yaml"
	// ConfigFileDev is the development configuration file
	ConfigFileDev = "config/config_dev.yaml"
)

// Config is the configuration structure for the NWDAF.
type Config struct {
	Mongo struct {
		Username   string `yaml:"username"`
		Password   string `yaml:"password"`
		URI        string `yaml:"uri"`
		Name       string `yaml:"name"`
		Collection string `yaml:"collection"`
	} `yaml:"mongo"`
	Server struct {
		Port   int    `yaml:"port"`
		BindIP string `yaml:"bind_ip"`
	} `yaml:"server"`
	TLS struct {
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
	} `yaml:"tls"`
	TrustedProxies []string    `yaml:"trusted_proxies"`
	LogLevel       hclog.Level `yaml:"log_level"`
	LogTag         string      `yaml:"log_tag"`
	PrometheusPort int         `yaml:"prometheus_port"`
	Redis          struct {
		URI      string `yaml:"uri"`
		Password string `yaml:"password"`
	} `yaml:"redis"`
	NRFIp    string                 `yaml:"nrf_uri"`
	CoreType plugin_shared.CoreType `yaml:"core_type"`
	Slices   []struct {
		ID             string   `yaml:"id"`
		Username       string   `yaml:"username"`
		Password       string   `yaml:"password"`
		CoreEndpointIp string   `yaml:"core_endpoint_ip"`
		AmfIPs         []string `yaml:"amf_ips"`
		SmfIPs         []string `yaml:"smf_ips"`
	} `yaml:"slices"`
}

// LoadConfig loads the configuration from a file
// If the filenamePointer is nil, it will try to load from the default path but if the development
// configuration file exists, it will use it.
// If the filenamePointer is not nil, it will load the configuration from the specified file.
func LoadConfig(filenamePointer *string) (*Config, error) {
	var filename string
	if filenamePointer == nil {
		if utils.FileExists(ConfigFileDev) {
			filename = ConfigFileDev
		} else {
			filename = ConfigFile
		}
	} else {
		filename = *filenamePointer
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
