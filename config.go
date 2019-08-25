package londo

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

// TODO: need a way to validate config file

type jwtParams struct {
	Issuer       string `yaml:"iss"`
	Audience     string `yaml:"aud"`
	ExpiresAfter int    `yaml:"exp"`
	Secret       string `yaml:"secret"`
}

type grpcConfig struct {
	Port int `yaml:"port"`
}

type certParams struct {
	Country             string `yaml:"country"`
	Province            string `yaml:"province"`
	Locality            string `yaml:"locality"`
	Organization        string `yaml:"organization"`
	StreetAddress       string `yaml:"street_address"`
	PostalCode          string `yaml:"postal_code"`
	OrgUnit             string `yaml:"organizational_unit"`
	OrgId               int    `yaml:"org_id"`
	Term                int    `yaml:"term"`
	BitSize             int    `yaml:"bit_size"`
	FormatType          string `yaml:"format_type"`
	CertType            int    `yaml:"cert_type"`
	MultiDomainCertType int    `yaml:"multi_domain_cert_type"`
	Comments            string `yaml:"comments"`
}

type endpoints struct {
	Revoke, Enroll, Collect string
}

type restApi struct {
	Url, Username, Password string
	CustomerURI             string `yaml:"customer_uri"`
	Endpoints               endpoints
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
	GRPC       grpcConfig `yaml:"grpc"`
	CertParams certParams `yaml:"cert_params"`
	Debug      int        `yaml:"debug"`
	JWT        jwtParams  `yaml:"jwt"`
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
