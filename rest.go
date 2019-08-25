package londo

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
)

const (
	contentType = "application/json"
)

type enrollReqBody struct {
	OrgId             int    `json:"orgId"`
	NumberServers     int    `json:"numberServers"`
	CertType          int    `json:"certType"`
	ServerType        int    `json:"serverType"`
	Term              int    `json:"term"`
	Csr               string `json:"csr"`
	SubjAltNames      string `json:"subjAltNames"`
	Comments          string `json:"comments"`
	ExternalRequester string `json:"externalRequester"`
}

type EnrollResponse struct {
	RenewID string `json:"renewId"`
	SslId   int    `json:"sslId"`
}

type ErrorResponse struct {
	Code        int    `json:"code"`
	Description string `json:"description"`
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
		SetHeader("login", r.config.Rest.Username).
		SetHeader("password", r.config.Rest.Password).
		SetHeader("customerUri", r.config.Rest.CustomerURI)
}

func (r RestAPI) Enroll(s *Subject) (*resty.Response, error) {
	var alts string

	certType := r.config.CertParams.CertType

	if len(s.AltNames) != 0 {
		for _, a := range s.AltNames {
			alts = alts + "," + a
		}

		certType = r.config.CertParams.MultiDomainCertType
	}

	body := enrollReqBody{
		OrgId:             r.config.CertParams.OrgId,
		Csr:               s.CSR,
		SubjAltNames:      alts,
		CertType:          certType,
		NumberServers:     0,
		ServerType:        -1,
		Term:              r.config.CertParams.Term,
		Comments:          r.config.CertParams.Comments,
		ExternalRequester: "",
	}

	j, _ := json.Marshal(&body)

	log.Debug("request: " + string(j))

	return r.request().
		SetBody(j).
		Post(r.config.Rest.Url +
			r.config.Rest.Endpoints.Enroll)
}

func (r RestAPI) Revoke(certId int) (*resty.Response, error) {
	return r.request().
		Post(r.config.Rest.Url +
			r.config.Rest.Endpoints.Revoke +
			"/" + strconv.Itoa(certId))
}

func (r RestAPI) Collect(certId int) (*resty.Response, error) {
	return r.request().
		Get(r.config.Rest.Url +
			r.config.Rest.Endpoints.Collect +
			"/" + strconv.Itoa(certId) + "/" + r.config.CertParams.FormatType)
}

func (r RestAPI) VerifyStatusCode(res *resty.Response, expected int) error {
	switch res.StatusCode() {
	case expected:
		return nil

	case http.StatusBadRequest:
		var e ErrorResponse
		if err := json.Unmarshal(res.Body(), &e); err != nil {
			return errors.New("bad request, cannot parse response")
		}
		return errors.New("bad request: " + e.Description)

	case http.StatusUnauthorized:
		return errors.New("unauthorized")

	case http.StatusInternalServerError:
		return errors.New("server error")

	case http.StatusNotFound:
		return errors.New("page not found, wrong endpoint")

	default:
		return errors.New("unhandled http error, status code: " + strconv.Itoa(res.StatusCode()))
	}
}
