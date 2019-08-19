package londo

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/streadway/amqp"
)

const (
	eventHeader = "x-event-name"

	RenewEventName     = "renew"
	RevokeEventName    = "revoke"
	EnrollEventName    = "enroll"
	DeleteSubjEvent    = "delete"
	CompleteEnrollName = "complete"
	CSREventName       = "newcsr"
	CollectEventName   = "collect"
)

type Londo struct {
	conn       *amqp.Connection
	exchange   string
	logChannel *LogChannel
	config     *Config
	db         *MongoDB
}

func (p *Londo) Shutdown() {
	p.conn.Close()
}

func NewMQConnection(c *Config, db *MongoDB, lch *LogChannel) (*Londo, error) {
	p := &Londo{
		exchange:   "londo-events",
		config:     c,
		logChannel: lch,
		db:         db,
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

func (p *Londo) ExchangeDeclare() error {
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

func (p *Londo) QueueDeclare(name string) (amqp.Queue, error) {
	ch, err := p.conn.Channel()
	if err != nil {
		return amqp.Queue{}, err
	}
	defer ch.Close()

	return ch.QueueDeclare(name, true, false, false, false, nil)
}

func (p *Londo) QueueBind(name string, key string) error {
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

func (p Londo) Consume(name string) {
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

func (p Londo) handleEvent(event string, body []byte) error {
	switch event {
	case RenewEventName:
		s, err := UnmarshalMsgBody(body)
		if err != nil {
			return err
		}

		// In the future, this procedure may change.
		// For now expiring certificates are being revoked before they are re-issued.
		err = p.handleRevokeRequest(s.CertID)
		if err != nil {
			return err
		}

		p.logChannel.Info <- "Revoked certificate: subject " + s.Subject + ", id: " + strconv.Itoa(s.CertID)

		err = p.Emit(DeleteSubjEvenet{}, &s)
		if err != nil {
			return err
		}
		p.logChannel.Info <- "Requested " + s.Subject + " to be deleted from database"

		err = p.Emit(EnrollEvent{}, &s)
		if err != nil {
			return err
		}
		p.logChannel.Info <- "Requested new enrollment for " + s.Subject

	case RevokeEventName:
		s, err := UnmarshalMsgBody(body)
		if err != nil {
			return err
		}

		p.logChannel.Info <- "Revoked certificate: subject " + s.Subject + ", id: " + strconv.Itoa(s.CertID)

	case EnrollEventName:
		s, err := UnmarshalMsgBody(body)
		if err != nil {
			return err
		}

		b, err := p.handEnrollRequest(&s)
		if err != nil {
			return err
		}

		var jr EnrollResponse
		err = json.Unmarshal(b, &jr)
		if err != nil {
			return err
		}

		s.CertID = jr.SslId
		s.OrderID = jr.RenewID

		err = p.Emit(CollectEvent{}, &s)
		if err != nil {
			return err
		}

		p.logChannel.Info <- "Enrolled new subject: " + s.Subject

	case DeleteSubjEvent:
		s, err := UnmarshalMsgBody(body)
		if err != nil {
			return err
		}

		if err = p.db.DeleteSubject(s.CertID); err != nil {
			return err
		}

		p.logChannel.Info <- "Deleted subject with id " + strconv.Itoa(s.CertID)

	default:
		return errors.New("unrecognized event type")
	}

	return nil
}

func (p Londo) handEnrollRequest(s *Subject) ([]byte, error) {
	c := NewRestClient(*p.config)
	res, err := c.Enroll(s)
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK {
		return nil, err
	}

	return res.Body(), nil
}

func (p Londo) handleRevokeRequest(id int) error {
	c := NewRestClient(*p.config)
	res, err := c.Revoke(id)
	if err != nil {
		return err
	}

	if res.StatusCode() != http.StatusCreated {
		return err
	}

	return nil
}

func (p Londo) parseEventHeader(h amqp.Table) (string, error) {
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

func (p Londo) getEventType(e Event, s *Subject) (Event, error) {
	switch e.(type) {
	case RenewEvent:
		return RenewEvent{
			Subject:  s.Subject,
			CertID:   s.CertID,
			AltNames: s.AltNames,
			Targets:  s.Targets,
		}, nil

	case RevokeEvent:
		return RevokeEvent{CertID: s.CertID}, nil

	case EnrollEvent:
		return EnrollEvent{
			Subject:  s.Subject,
			AltNames: s.AltNames,
			Targets:  s.Targets,
		}, nil

	case DeleteSubjEvenet:
		return DeleteSubjEvenet{CertID: s.CertID}, nil

	case CSREvent:
		return CSREvent{
			Subject:    s.Subject,
			CSR:        s.CSR,
			PrivateKey: s.PrivateKey,
			AltNames:   s.AltNames,
			Targets:    s.Targets,
		}, nil

	case CompleteEnrollEvent:
		return CompleteEnrollEvent{
			Subject:     s.Subject,
			CertID:      s.CertID,
			OrderID:     s.OrderID,
			Certificate: s.Certificate,
		}, nil

	case CollectEvent:
		return CollectEvent{CertID: s.CertID}, nil

	default:
		return nil, errors.New("unknown event type")
	}
}

func (p Londo) Emit(e Event, s *Subject) error {

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
