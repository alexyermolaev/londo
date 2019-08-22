package londo

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

type certParams struct {
	Country, Province, Locality, Organization string
	StreetAddress                             string `yaml:"street_address"`
	PostalCode                                string `yaml:"postal_code"`
	OrgUnit                                   string `yaml:"organizational_unit"`
	Term                                      int
	BitSize                                   int    `yaml:"bit_size"`
	FormatType                                string `yaml:"format_type"`
	CertType                                  int    `yaml:"cert_type"`
	MultiDomainCertType                       int    `yaml:"multi_domain_cert_type"`
}

type endpoints struct {
	Revoke, Enroll, Collect string
}

type restApi struct {
	Url, Username, Password, CustomerURI string
	Endpoints                            endpoints
}

type rabbitmq struct {
	Hostname, Username, Password, Exchange string
	Port                                   int
}

type db struct {
	Hostname, Username, Password, Name string
	Port                               int
}

type Config struct {
	DB         db         `yaml:"mongodb"`
	AMQP       rabbitmq   `yaml:"amqp"`
	RestAPI    restApi    `yaml:"sectigo"`
	CertParams certParams `yaml:"cert_params"`
	Debug      int        `yaml:"debug"`
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
