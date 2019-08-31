package londo

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"math/big"
	"strconv"
)

const (
	CsrType        = "CERTIFICATE REQUEST"
	PrivateKeyType = "PRIVATE KEY"
	PublicKeyType  = "CERTIFICATE"
)

func ParsePublicCertificate(c string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(c))

	if block == nil || block.Type != PublicKeyType {
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

func GetCertSerialNumber(ip string, port int) (*big.Int, error) {
	conn, err := tls.Dial("tcp", ip+":"+strconv.Itoa(port), &tls.Config{})
	if err != nil {
		return nil, err
	}

	return conn.ConnectionState().PeerCertificates[0].SerialNumber, nil
}
