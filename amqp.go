package londo

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/streadway/amqp"
)

const (
	eventHeader = "x-event-name"
	renewEvent  = "renew"
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
		"amqp://" + c.AMQP.Username + ":" + c.AMQP.Password + "@" + c.AMQP.Hostname + ":" +
			strconv.Itoa(c.AMQP.Port))
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

	return ch.ExchangeDeclare(
		p.exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil)
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

	if err = ch.QueueBind(
		name,
		key,
		p.exchange,
		false,
		nil,
	); err != nil {
		return err
	}

	return nil
}

func (p AMQP) Consume(name string) error {
	ch, err := p.conn.Channel()
	if err != nil {
		return err
	}

	deliver, err := ch.Consume(
		name, "",
		false,
		false,
		false,
		false,
		nil)
	if err != nil {
		return err
	}

	for d := range deliver {
		event, err := p.parseEventHeader(d.Headers)
		if err != nil {
			logrus.Warn(err)
			continue
		}

		switch event {
		case renewEvent:
			var e RenewEvent
			err := json.Unmarshal(d.Body, &e)
			if err != nil {
				logrus.Warn("cannot unmarshal message")
				continue
			}
			logrus.Info(e.CertID)
		default:
			logrus.Warn("Unrecognized event type. Ignoring")
			continue
		}

		d.Ack(false)
	}

	err = ch.Cancel("", true)
	if err != nil {
		return err
	}

	return nil
}

func (p AMQP) parseEventHeader(h amqp.Table) (string, error) {
	raw, ok := h[eventHeader]
	if !ok {
		return "", errors.New("no " + eventHeader + " header found")
	}
	e, ok := raw.(string)
	if !ok {
		return "", errors.New("event name is not a string")
	}
	return e, nil
}

func (p AMQP) getEventType(e Event, s *Subject) (Event, error) {
	switch e.(type) {
	case RenewEvent:
		return RenewEvent{
			Subject: s.Subject,
			CertID:  s.CertID,
		}, nil
	default:
		return nil, errors.New("unknown event type")
	}
}

func (p AMQP) Emit(e Event, s *Subject) error {

	event, err := p.getEventType(e, s)

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
		Headers:     amqp.Table{eventHeader: event.EventName()},
		ContentType: "application/json",
		Body:        j,
	}

	return ch.Publish(
		p.exchange,
		event.EventName(),
		false,
		false,
		msg)
}

type Event interface {
	EventName() string
}

type RenewEvent struct {
	Subject string
	CertID  int
}

func (e RenewEvent) EventName() string {
	return renewEvent
}
