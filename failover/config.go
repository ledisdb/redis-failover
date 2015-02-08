package failover

import (
	"github.com/BurntSushi/toml"
	"io/ioutil"
)

const (
	ClusterStateNew      = "new"
	ClusterStateExisting = "existing"
)

type Config struct {
	Addr         string   `toml:"addr"`
	ServerID     int      `toml:"server_id"`
	Cluster      []string `toml:"cluster"`
	ClusterState string   `toml:"cluster_state"`
	DataDir      string   `toml:"data_dir"`
	LogDir       string   `toml:"log_dir"`
}

func NewConfigWithFile(name string) (*Config, error) {
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}

	return NewConfig(string(data))
}

func NewConfig(data string) (*Config, error) {
	var c Config

	_, err := toml.Decode(data, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}
