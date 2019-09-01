package londo

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

// TODO: need a way to validate config file

type JWT struct {
	Issuer       string `yaml:"iss"`
	Audience     string `yaml:"aud"`
	ExpiresAfter int    `yaml:"exp"`
	Secret       string `yaml:"secret"`
}

type GRPC struct {
	Port int `yaml:"port"`
}

type CertParams struct {
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

type Rest struct {
	Url, Username, Password string
	CustomerURI             string `yaml:"customer_uri"`
	Endpoints               endpoints
}

type rabbitmq struct {
	Hostname, Username, Password, Exchange string
	Port                                   int
}

type DB struct {
	Hostname, Username, Password, Name string
	Port                               int
}

type Config struct {
	DB         `yaml:"mongodb"`
	AMQP       rabbitmq `yaml:"amqp"`
	Rest       `yaml:"sectigo"`
	GRPC       `yaml:"grpc"`
	CertParams `yaml:"cert_params"`
	Debug      int `yaml:"debug"`
	JWT        `yaml:"jwt"`
}

func ReadConfig(file string) (*Config, error) {

	f, err := os.Open(file)
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
