package londo

import (
	"strconv"

	"github.com/go-resty/resty/v2"
)

const (
	contentType = "application/json"
)

type RestAPI struct {
	Client *resty.Client
	config *Config
}

func NewRestClient(c Config) *RestAPI {
	return &RestAPI{Client: resty.New()}
}

func (r RestAPI) request() *resty.Request {
	return r.Client.R().
		SetHeader("Content-Type", contentType).
		SetHeader("login", r.config.RestAPI.Username).
		SetHeader("password", r.config.RestAPI.Password).
		SetHeader("customerUri", r.config.RestAPI.CustomerURI)
}

func (r RestAPI) Revoke(certid int) (*resty.Response, error) {
	return r.request().
		Post("https://" +
			r.config.RestAPI.Url +
			r.config.RestAPI.Endpoints.Revoke +
			"/" + strconv.Itoa(certid))
}
