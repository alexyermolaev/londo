package londo

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"os"
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

type AMQP struct {
	conn       *amqp.Connection
	exchange   string
	logChannel *LogChannel
	config     *Config
	db         *MongoDB
}

func (a *AMQP) Shutdown() {
	a.conn.Close()
}

func NewMQConnection(c *Config, db *MongoDB, lch *LogChannel) (*AMQP, error) {
	p := &AMQP{
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

func (a *AMQP) ExchangeDeclare() error {
	ch, err := a.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return ch.ExchangeDeclare(
		a.exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil)
}

func (a *AMQP) QueueDeclare(name string) (amqp.Queue, error) {
	ch, err := a.conn.Channel()
	if err != nil {
		return amqp.Queue{}, err
	}
	defer ch.Close()

	return ch.QueueDeclare(name, true, false, false, false, nil)
}

func (a *AMQP) QueueBind(name string, key string) error {
	ch, err := a.conn.Channel()
	if err != nil {
		return err
	}

	if err = ch.QueueBind(
		name,
		key,
		a.exchange,
		false,
		nil,
	); err != nil {
		return err
	}

	return nil
}

func (a AMQP) Consume(name string) {
	ch, err := a.conn.Channel()
	if err != nil {
		a.logChannel.Abort <- err
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
		a.logChannel.Abort <- err
		return
	}

	for d := range deliver {
		event, err := a.parseEventHeader(d.Headers)
		if err != nil {
			a.logChannel.Err <- err
			continue
		}

		err = a.handleEvent(event, d.Body)
		if err != nil {
			a.logChannel.Err <- err
			continue
		}

		d.Ack(false)
	}

	err = ch.Cancel("", true)
	if err != nil {
		a.logChannel.Err <- err
	}
}

func (a AMQP) handleEvent(event string, body []byte) error {
	switch event {
	case RenewEventName:
		s, err := UnmarshalMsgBody(body)
		if err != nil {
			return err
		}

		// In the future, this procedure may change.
		// For now expiring certificates are being revoked before they are re-issued.
		err = a.handleRevokeRequest(s.CertID)
		if err != nil {
			return err
		}

		a.logChannel.Info <- "Revoked certificate: subject " + s.Subject + ", id: " + strconv.Itoa(s.CertID)

		err = a.Emit(DeleteSubjEvenet{}, &s)
		if err != nil {
			return err
		}
		a.logChannel.Info <- "Requested " + s.Subject + " to be deleted from database"

		err = a.Emit(EnrollEvent{}, &s)
		if err != nil {
			return err
		}
		a.logChannel.Info <- "Requested new enrollment for " + s.Subject

	case RevokeEventName:
		s, err := UnmarshalMsgBody(body)
		if err != nil {
			return err
		}

		a.logChannel.Info <- "Revoked certificate: subject " + s.Subject + ", id: " + strconv.Itoa(s.CertID)

	case EnrollEventName:
		s, err := UnmarshalMsgBody(body)
		if err != nil {
			return err
		}

		key, err := GeneratePrivateKey(a.config.CertParams.BitSize)
		if err != nil {
			return err
		}

		csrBytes, err := GenerateCSR(key, "", a.config)
		if err != nil {
			return err
		}

		csr := &pem.Block{
			Type:    "CERTIFICATE REQUEST",
			Headers: nil,
			Bytes:   csrBytes,
		}

		err = pem.Encode(os.Stdout, csr)

		if err = pem.Encode(os.Stdout, csr); err != nil {
			return err
		}

		//b, err := a.handEnrollRequest(&s)
		//if err != nil {
		//	return err
		//}
		//
		//var jr EnrollResponse
		//err = json.Unmarshal(b, &jr)
		//if err != nil {
		//	return err
		//}
		//
		//s.CertID = jr.SslId
		//s.OrderID = jr.RenewID
		//
		//err = a.Emit(CollectEvent{}, &s)
		//if err != nil {
		//	return err
		//}

		a.logChannel.Info <- "Enrolled new subject: " + s.Subject

	case DeleteSubjEvent:
		s, err := UnmarshalMsgBody(body)
		if err != nil {
			return err
		}

		if err = a.db.DeleteSubject(s.CertID); err != nil {
			return err
		}

		a.logChannel.Info <- "Deleted subject with id " + strconv.Itoa(s.CertID)

	default:
		return errors.New("unrecognized event type")
	}

	return nil
}

func (a AMQP) handEnrollRequest(s *Subject) ([]byte, error) {
	c := NewRestClient(*a.config)
	res, err := c.Enroll(s)
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != http.StatusOK {
		return nil, err
	}

	return res.Body(), nil
}

func (a AMQP) handleRevokeRequest(id int) error {
	c := NewRestClient(*a.config)
	res, err := c.Revoke(id)
	if err != nil {
		return err
	}

	if res.StatusCode() != http.StatusCreated {
		return err
	}

	return nil
}

func (a AMQP) parseEventHeader(h amqp.Table) (string, error) {
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

func (a AMQP) getEventType(e Event, s *Subject) (Event, error) {
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

func (a AMQP) Emit(e Event, s *Subject) error {

	event, err := a.getEventType(e, s)

	j, err := json.Marshal(&event)
	if err != nil {
		return err
	}

	ch, err := a.conn.Channel()
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
		a.exchange,
		event.EventName(),
		false,
		false,
		msg)
}
