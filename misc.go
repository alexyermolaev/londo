package londo

import (
	"encoding/json"
	"github.com/streadway/amqp"
)

func UnmarshalMsg(d *amqp.Delivery) (Subject, error) {
	var s Subject
	if err := json.Unmarshal(d.Body, &s); err != nil {
		d.Reject(false)
	}
	return s, nil
}
