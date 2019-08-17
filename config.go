package londo

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

type dbConfig struct {
	hostname string
	port     int
	username string
	password string
}

type Config struct {
	DB dbConfig `yaml:"mongodb"`
}

func ReadConfig() (*Config, error) {
	// path for now
	p := "config/config.yaml"

	f, err := os.Open(p)
	defer f.Close()
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	c := Config{}

	if err = yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}

	return &c, nil
}
