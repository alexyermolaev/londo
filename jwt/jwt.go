package jwt

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/gbrlsnchs/jwt/v3"
)

var (
	Secret *jwt.HMACSHA
)

type Payload struct {
	jwt.Payload
}

func IssueJWT(sub string) ([]byte, error) {
	now := time.Now()
	exp := now.Add(48 * time.Hour)

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

func ReadSecret(sfile string) {
	fi, err := os.Lstat(sfile)
	if err != nil {
		log.Fatal(err)
	}

	if fi.Mode().Perm() != 256 {
		log.Fatal(errors.New("perm " + sfile + ": should be 0400"))
	}

	s, err := ioutil.ReadFile(sfile)
	if err != nil {
		log.Fatal(err)
	}

	Secret = jwt.NewHS512(s)
}
