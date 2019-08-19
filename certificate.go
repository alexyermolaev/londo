package londo

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
)

func ParsePublicCertificate(s Subject) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(s.Certificate))
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("failed to decode PEM block containing public key")
	}

	return x509.ParseCertificate(block.Bytes)
}

func GeneratePrivateKey(bs int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bs)
}

func GenerateCSR(key crypto.PrivateKey, cn string, c *Config) ([]byte, error) {
	subj := pkix.Name{
		Country:            []string{c.CertParams.Country},
		Organization:       []string{c.CertParams.Organization},
		OrganizationalUnit: []string{c.CertParams.OrgUnit},
		Locality:           []string{c.CertParams.Locality},
		Province:           []string{c.CertParams.Province},
		StreetAddress:      []string{c.CertParams.StreetAddress},
		PostalCode:         []string{c.CertParams.PostalCode},
		SerialNumber:       "",
		CommonName:         cn,
		Names:              nil,
		ExtraNames:         nil,
	}

	rawSubj := subj.ToRDNSequence()
	asn1Subj, err := asn1.Marshal(rawSubj)
	if err != nil {
		return nil, err
	}

	tpl := x509.CertificateRequest{
		RawSubject: asn1Subj,
	}

	return x509.CreateCertificateRequest(rand.Reader, &tpl, key)
}
