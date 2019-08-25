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

func IssueJWT(sub string) ([]byte, error) {
	exp := time.Time(24 * Config.JWT.ExpiresAfter * time.Hour)

	pl := Payload{
		Payload: jwt.Payload{
			Issuer:         Config.JWT.Issuer,
			Subject:        sub,
			Audience:       jwt.Audience(Config.JWT.Audience),
			ExpirationTime: jwt.NumericDate(exp),
			IssuedAt:       jwt.NumericDate(now),
			NotBefore:      jwt.NumericDate(now),
		},
	}

	return jwt.Sign(pl, hs)
}

func Verify(token []byte) error {
	var (
		expValid   = jwt.ExpirationTimeValidator(now)
		pl         Payload
		valPayload = jwt.ValidatePayload(&pl.Payload, expValid)
	)

	_, err := jwt.Verify(token, hs, valPayload)
	if err != nil {
		return err
	}

	return nil
}
