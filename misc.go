package londo

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/streadway/amqp"
	"google.golang.org/grpc/peer"
)

func UnmarshalSubjMsg(d *amqp.Delivery) (Subject, error) {
	var s Subject
	if err := json.Unmarshal(d.Body, &s); err != nil {
		_ = d.Reject(false)
	}
	return s, nil
}

func ParseIPAddr(ctx context.Context) (string, string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", "", err
	}

	ip := strings.FieldsFunc(p.Addr.String(), func(r rune) bool {
		return r == ':'
	})[0]

	return ip, p.Addr.String(), nil
}
