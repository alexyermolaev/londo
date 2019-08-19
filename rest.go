package londo

import (
	"strconv"

	"github.com/go-resty/resty/v2"
)

const (
	contentType = "application/json"
)

type enrollBody struct {
	orgId             int
	csr               string
	subjAltNames      string
	certType          int
	numberServers     int
	serverType        int
	term              int
	comments          string
	externalRequester string
}

type EnrollResponse struct {
	RenewID string `json:"renewId"`
	SslId   int    `json:"sslId"`
}

type RestAPI struct {
	Client *resty.Client
	config *Config
}

func NewRestClient(c Config) *RestAPI {
	return &RestAPI{
		Client: resty.New(),
		config: &c,
	}
}

func (r RestAPI) request() *resty.Request {
	return r.Client.R().
		SetHeader("Content-Type", contentType).
		SetHeader("login", r.config.RestAPI.Username).
		SetHeader("password", r.config.RestAPI.Password).
		SetHeader("customerUri", r.config.RestAPI.CustomerURI)
}

func (r RestAPI) Enroll(s *Subject) (*resty.Response, error) {
	var alts string

	for _, a := range s.AltNames {
		alts = alts + "," + a
	}
	return r.request().
		SetBody(enrollBody{
			orgId:             1,
			csr:               s.CSR,
			subjAltNames:      alts,
			certType:          1729,
			numberServers:     0,
			serverType:        -1,
			term:              r.config.CSR.Term,
			comments:          "automated request by londo",
			externalRequester: "",
		}).
		Post(r.config.RestAPI.Url +
			r.config.RestAPI.Endpoints.Enroll)
}

func (r RestAPI) Revoke(certid int) (*resty.Response, error) {
	return r.request().
		Post(r.config.RestAPI.Url +
			r.config.RestAPI.Endpoints.Revoke +
			"/" + strconv.Itoa(certid))
}
