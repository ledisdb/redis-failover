package failover

import (
	"github.com/BurntSushi/toml"
	"io/ioutil"
)

type Config struct {
	Addr    string   `toml:"addr"`
	Cluster []string `toml:"cluster"`
	DataDir string   `toml:"data_dir"`
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
