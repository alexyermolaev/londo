package londo

import (
	"crypto/x509"
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
