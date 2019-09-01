package jwt

import (
	"time"

	"github.com/gbrlsnchs/jwt/v3"
)

var (
	Secret *jwt.HMACSHA
)

type Payload struct {
	jwt.Payload
}

func EncryptSecret(s []byte) {
	Secret = jwt.NewHS512(s)
}

func IssueJWT(sub string) ([]byte, error) {
	now := time.Now()
	exp := now.Add(12 * time.Hour)

	pl := Payload{
		Payload: jwt.Payload{
			Subject:        sub,
			ExpirationTime: jwt.NumericDate(exp),
			IssuedAt:       jwt.NumericDate(now),
			NotBefore:      jwt.NumericDate(now),
		},
	}

	return jwt.Sign(pl, Secret)
}

func VerifyJWT(token []byte) (string, error) {
	var (
		pl Payload

		now = time.Now()
		// TODO: Other validators
		expValid = jwt.ExpirationTimeValidator(now)

		valPayload = jwt.ValidatePayload(&pl.Payload, expValid)
	)

	_, err := jwt.Verify(token, Secret, &pl, valPayload)
	if err != nil {
		return pl.Subject, err
	}

	return pl.Subject, nil
}
