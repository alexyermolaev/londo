package londo

import (
	"encoding/json"
	"strconv"

	"github.com/streadway/amqp"
)

type Producer struct {
	conn     *amqp.Connection
	exchange string
}

func (p *Producer) Shutdown() {
	p.conn.Close()
}

func NewProducer(c *Config) (*Producer, error) {
	p := &Producer{
		exchange: "londo-events",
	}

	var err error

	p.conn, err = amqp.Dial(
		"amqp://" + c.AMQP.Username + ":" + c.AMQP.Password + "@" + c.AMQP.Hostname + ":" + strconv.Itoa(c.AMQP.Port))
	if err != nil {
		return p, err
	}

	ch, err := p.conn.Channel()
	if err != nil {
		return p, err
	}
	defer ch.Close()

	return p, ch.ExchangeDeclare(p.exchange, "topic", true, false, false, false, nil)
}

func (p Producer) EmitRenew(s *Subject) error {
	event := RenewEvent{
		Subject: s.Subject,
		CertID:  s.CertID,
	}

	j, err := json.Marshal(&event)
	if err != nil {
		return err
	}

	ch, err := p.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	msg := amqp.Publishing{
		Headers:     amqp.Table{"x-event-name": event.EventName()},
		ContentType: "application/json",
		Body:        j,
	}

	return ch.Publish(p.exchange, event.EventName(), false, false, msg)
}

type RenewEvent struct {
	Subject string
	CertID  int
}

func (e RenewEvent) EventName() string {
	return "renew"
}
