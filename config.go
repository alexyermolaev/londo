package londo

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

type certParams struct {
	Country       string
	Province      string
	Locality      string
	StreetAddress string
	PostalCode    string
	Organization  string
	OrgUnit       string `yaml:"organizational_unit"`
	Term          int
	BitSize       int
	FormatType    string
}

type endpoints struct {
	Revoke string
	Enroll string
}

type restapi struct {
	Url         string
	Username    string
	Password    string
	CustomerURI string
	Endpoints   endpoints
}

type rabbitmq struct {
	Hostname string
	Port     int
	Username string
	Password string
	Exchange string
}

type db struct {
	Hostname string
	Port     int
	Username string
	Password string
	Name     string
}

type Config struct {
	DB         db         `yaml:"mongodb"`
	AMQP       rabbitmq   `yaml:"amqp"`
	RestAPI    restapi    `yaml:"sectigo"`
	CertParams certParams `yaml:"csr"`
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
