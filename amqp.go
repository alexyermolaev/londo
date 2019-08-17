package londo

import (
	"encoding/json"
	"strconv"

	"github.com/streadway/amqp"
)

type Producer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
	done    chan error
}

func (p *Producer) Shutdown() error {
	if err := p.channel.Cancel(p.tag, true); err != nil {
		return err
	}
	return nil
}

func NewProducer(c *Config) (*Producer, error) {
	p := &Producer{
		done: make(chan error),
	}

	var err error

	p.conn, err = amqp.Dial(
		"amqp://" + c.AMQP.Username + ":" + c.AMQP.Password + "@" + c.AMQP.Hostname + ":" + strconv.Itoa(c.AMQP.Port))
	if err != nil {
		return p, err
	}
	defer p.conn.Close()

	p.channel, err = p.conn.Channel()
	if err != nil {
		return p, err
	}

	if err := p.channel.ExchangeDeclare(
		"londo-events", "topic", true, false, false, false, nil); err != nil {
		return p, err
	}

	return p, err
}

func (p Producer) EmitRenew(s *Subject) error {

	msg := RenewEvent{
		Subject: s.Subject,
		CertID:  s.CertID,
	}

	j, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if err := p.channel.Publish(
		"londo-events", "renew", false, false, amqp.Publishing{
			Headers:         amqp.Table{},
			ContentType:     "application/json",
			ContentEncoding: "",
			Body:            j,
			DeliveryMode:    amqp.Transient,
			Priority:        0,
		},
	); err != nil {
		return err
	}
	return nil
}

type RenewEvent struct {
	Subject string
	CertID  int
}
