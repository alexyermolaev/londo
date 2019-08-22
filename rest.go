package londo

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-resty/resty/v2"
)

const (
	contentType = "application/json"
)

type enrollReqBody struct {
	orgId, numberServers, certType, serverType, term int
	csr, subjAltNames, comments, externalRequester   string
}

type EnrollResponse struct {
	RenewID string `json:"renewId"`
	SslId   int    `json:"sslId"`
}

type RestAPI struct {
	Client *resty.Client
	config *Config
}

func NewRestClient(c *Config) *RestAPI {
	return &RestAPI{
		Client: resty.New(),
		config: c,
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

	certType := r.config.CertParams.CertType

	if s.AltNames != nil {
		for _, a := range s.AltNames {
			alts = alts + "," + a
		}

		certType = r.config.CertParams.MultiDomainCertType
	}

	body := enrollReqBody{
		orgId:             1,
		csr:               s.CSR,
		subjAltNames:      alts,
		certType:          certType,
		numberServers:     0,
		serverType:        -1,
		term:              r.config.CertParams.Term,
		comments:          "automated request by londo",
		externalRequester: "",
	}

	j, _ := json.Marshal(&body)

	return r.request().
		SetBody(j).
		Post(r.config.RestAPI.Url +
			r.config.RestAPI.Endpoints.Enroll)
}

func (r RestAPI) Revoke(certId int) (*resty.Response, error) {
	return r.request().
		Post(r.config.RestAPI.Url +
			r.config.RestAPI.Endpoints.Revoke +
			"/" + strconv.Itoa(certId))
}

func (r RestAPI) Collect(certId int) (*resty.Response, error) {
	return r.request().
		Get(r.config.RestAPI.Url +
			r.config.RestAPI.Endpoints.Collect +
			"/" + strconv.Itoa(certId) + "/" + r.config.CertParams.FormatType)
}

func (r RestAPI) VerifyStatusCode(res *resty.Response, expected int) error {
	switch res.StatusCode() {
	case http.StatusUnauthorized:
		return errors.New("unauthorized")
	case http.StatusInternalServerError:
		return errors.New("server error")
	case http.StatusNotFound:
		return errors.New("page not found, wrong endpoint")
	case expected:
		return nil
	default:
		return errors.New("unhandled http error")
	}
}
