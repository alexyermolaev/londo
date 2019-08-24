package londo

import (
	"encoding/json"
	"strings"

	"github.com/streadway/amqp"
)

func UnmarshalSubjMsg(d *amqp.Delivery) (Subject, error) {
	var s Subject
	if err := json.Unmarshal(d.Body, &s); err != nil {
		_ = d.Reject(false)
	}
	return s, nil
}

func GetIPAddr(addr string) string {
	return strings.FieldsFunc(addr, func(r rune) bool {
		return r == ':'
	})[0]
}
