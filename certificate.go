package londo

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
)

const (
	CsrType        = "CERTIFICATE REQUEST"
	PrivateKeyType = "PRIVATE KEY"
	PublickKeyType = "CERTIFICATE"
)

func ParsePublicCertificate(c string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(c))
	if block == nil || block.Type != PublickKeyType {
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

func EncodeCSR(b []byte) (string, error) {
	block := &pem.Block{
		Type:  CsrType,
		Bytes: b,
	}

	return encodeBuffer(block)
}

func EncodePKey(key *rsa.PrivateKey) (string, error) {
	block := &pem.Block{
		Type:  PrivateKeyType,
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	return encodeBuffer(block)
}

func encodeBuffer(block *pem.Block) (string, error) {

	buf := new(bytes.Buffer)

	if err := pem.Encode(buf, block); err != nil {
		return "", err
	}
	return buf.String(), nil
}
