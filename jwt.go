package londo

import (
	"time"

	"github.com/gbrlsnchs/jwt/v3"
)

type Payload struct {
	jwt.Payload
}

var (
	hs  = jwt.NewHS512([]byte("secret"))
	now = time.Now()
)

func IssueJWT(sub string, c *Config) ([]byte, error) {
	exp := time.Time(now.Add(24 * 30 * time.Hour))

	pl := Payload{
		Payload: jwt.Payload{
			Issuer:         c.JWT.Issuer,
			Subject:        sub,
			ExpirationTime: jwt.NumericDate(exp),
			IssuedAt:       jwt.NumericDate(now),
			NotBefore:      jwt.NumericDate(now),
		},
	}

	return jwt.Sign(pl, hs)
}

func VerifyJWT(token []byte, c *Config) (string, error) {
	var (
		pl Payload

		// TODO: Other validators
		expValid = jwt.ExpirationTimeValidator(now)

		valPayload = jwt.ValidatePayload(&pl.Payload, expValid)
	)

	_, err := jwt.Verify(token, hs, valPayload)
	if err != nil {
		return pl.Subject, err
	}

	return pl.Subject, nil
}
