package londo

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/streadway/amqp"
)

const (
	eventHeader     = "x-event-name"
	RenewEventName  = "renew"
	RevokeEventName = "revoke"
)

type AMQP struct {
	conn       *amqp.Connection
	exchange   string
	logChannel *LogChannel
	config     *Config
}

func (p *AMQP) Shutdown() {
	p.conn.Close()
}

func NewMQConnection(c *Config, lch *LogChannel) (*AMQP, error) {
	p := &AMQP{
		exchange:   "londo-events",
		config:     c,
		logChannel: lch,
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

func (p AMQP) Consume(name string) {
	ch, err := p.conn.Channel()
	if err != nil {
		p.logChannel.Abort <- err
		return
	}

	deliver, err := ch.Consume(
		name, "",
		false,
		false,
		false,
		false,
		nil)
	if err != nil {
		p.logChannel.Abort <- err
		return
	}

	for d := range deliver {
		event, err := p.parseEventHeader(d.Headers)
		if err != nil {
			p.logChannel.Err <- err
			continue
		}

		err = p.handleEvent(event, d.Body)
		if err != nil {
			p.logChannel.Err <- err
			continue
		}

		d.Ack(false)
	}

	err = ch.Cancel("", true)
	if err != nil {
		p.logChannel.Err <- err
	}
}

func (p AMQP) handleEvent(event string, body []byte) error {
	switch event {
	case RenewEventName:
		var s Subject
		err := json.Unmarshal(body, &s)
		if err != nil {
			logrus.Warn("cannot unmarshal message")
			return err
		}

		err = p.handleRevoke(s.CertID)
		if err != nil {
			return err
		}

		p.logChannel.Info <- "Revoked certificate ID: " + strconv.Itoa(s.CertID)

	case RevokeEventName:
		var s Subject
		err := json.Unmarshal(body, &s)
		if err != nil {
			logrus.Warn("cannot unmarshal message")
		}

		p.logChannel.Info <- "Revoking " + strconv.Itoa(s.CertID)

	default:
		return errors.New("unrecognized event type")
	}

	return nil
}

func (p AMQP) handleRevoke(id int) error {
	c := NewRestClient(*p.config)
	resp, err := c.Revoke(id)
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusCreated {
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
	case RevokeEvent:
		return RevokeEvent{
			CertID: s.CertID,
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
	return RenewEventName
}

type RevokeEvent struct {
	CertID int
}

func (e RevokeEvent) EventName() string {
	return RevokeEventName
}
