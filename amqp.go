package londo

import (
	"encoding/json"
	"strconv"

	"github.com/streadway/amqp"
)

type AMQP struct {
	conn     *amqp.Connection
	exchange string
}

func (p *AMQP) Shutdown() {
	p.conn.Close()
}

func NewMQConnection(c *Config) (*AMQP, error) {
	p := &AMQP{
		exchange: "londo-events",
	}

	var err error

	p.conn, err = amqp.Dial(
		"amqp://" + c.AMQP.Username + ":" + c.AMQP.Password + "@" + c.AMQP.Hostname + ":" + strconv.Itoa(c.AMQP.Port))
	if err != nil {
		return p, err
	}

	return p, err
}

func (p *AMQP) ExchangeDeclare() error {
	ch, err := p.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return ch.ExchangeDeclare(p.exchange, "topic", true, false, false, false, nil)
}

func (p *AMQP) QueueDeclare(name string) (amqp.Queue, error) {
	ch, err := p.conn.Channel()
	if err != nil {
		return amqp.Queue{}, err
	}
	defer ch.Close()

	return ch.QueueDeclare(name, true, false, false, false, nil)
}

func (p *AMQP) QueueBind(name string, key string) error {
	ch, err := p.conn.Channel()
	if err != nil {
		return err
	}

	if err = ch.QueueBind(name, key, p.exchange, false, nil); err != nil {
		return err
	}

	return nil
}

func (p AMQP) EmitRenew(s *Subject) error {
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
