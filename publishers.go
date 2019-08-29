package londo

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

func (l *Londo) Publish(e interface{}, reply string, cmd string) error {
	var (
		ex, q string
	)

	j, err := json.Marshal(&e)
	if err != nil {
		return err
	}

	msg := amqp.Publishing{
		ContentType: ContentType,
		Body:        j,
	}

	switch e.(type) {
	case CollectEvent:
		ex = CollectExchange
		q = CollectQueue

	case CheckDNSEvent:
		ex = DbReplyExchange
		q = DbReplyQueue

	case GetExpringSubjEvent:
		ex = DbReplyExchange
		q = DbReplyQueue
		msg.Type = DbGetExpiringSubjectsCmd
		msg.ReplyTo = reply

	case EnrollEvent:
		ex = EnrollExchange
		q = EnrollQueue

	case RenewEvent:
		ex = RenewExchange
		q = RenewQueue
		msg.CorrelationId = e.(RenewEvent).ID
		msg.Expiration = strconv.Itoa(int(time.Now().Add(1 * time.Minute).Unix()))

	case Subject:
		ex = GRPCServerExchange
		q = reply
		msg.Type = cmd

	default:
		return errors.New("unknown event")
	}

	if err := l.AMQP.Emit(ex, q, msg); err != nil {
		return err
	}

	return nil
}

func (l *Londo) PublishDbCommand(s *Subject, cmd string, reply string) *Londo {
	var logMsg string
	var e interface{}

	switch cmd {
	case DbGetSubjectByTargetCmd:
		e = GetSubjectByTarget{Target: s.Targets}

	case DbGetSubjectCmd:
		e = GetSubjectEvenet{Subject: s.Subject}

	case DbAddSubjCmd:
		e = NewSubjectEvenet{
			Subject:    s.Subject,
			CSR:        s.CSR,
			PrivateKey: s.PrivateKey,
			CertID:     s.CertID,
			OrderID:    s.OrderID,
			AltNames:   s.AltNames,
			Targets:    s.Targets,
		}

		logMsg = "letting db know that " + s.Subject + " needs to be created."

	case DbUpdateSubjCmd:
		e = CompleteEnrollEvent{
			CertID:      s.CertID,
			Certificate: s.Certificate,
		}

		logMsg = "letting db know that " + strconv.Itoa(s.CertID) + " needs to be updated with a certificate."

	default:
		log.Errorf("received unknown db command: %s", cmd)
		return l
	}

	j, err := json.Marshal(&e)
	if err != nil {
		log.Error(err)
		return l
	}

	if err := l.AMQP.Emit(
		DbReplyExchange,
		DbReplyQueue,
		amqp.Publishing{
			ContentType: ContentType,
			Type:        cmd,
			ReplyTo:     reply,
			Body:        j,
		}); err != nil {
		log.Error(err)
		return l
	}

	log.Info(logMsg)

	return l
}
