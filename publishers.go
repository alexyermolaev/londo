package londo

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/roylee0704/gron"
	"github.com/streadway/amqp"
)

func (l *Londo) PublishExpiringCerts(exchange string, queue string, reply string) *Londo {
	cron := gron.New()

	cron.AddFunc(gron.Every(1*time.Minute), func() {
		exp, err := l.Db.FindExpiringSubjects(720)
		CheckFatalError(err)

		for _, e := range exp {

			re := RenewEvent{
				Subject:  e.Subject,
				CertID:   e.CertID,
				AltNames: e.AltNames,
				Targets:  e.Targets,
			}

			j, err := json.Marshal(&re)
			if err != nil {
				l.Log.Err <- err
				return
			}

			if err = l.AMQP.Emit(
				exchange,
				queue,
				amqp.Publishing{
					ContentType:   ContentType,
					ReplyTo:       reply,
					CorrelationId: e.ID.Hex(),
					Expiration:    strconv.Itoa(int(time.Now().Add(1 * time.Minute).Unix())),
					Body:          j,
				}); err != nil {
				l.Log.Err <- err
				return
			}
			l.Log.Info <- "published " + e.Subject
		}
	})

	cron.Start()

	return l
}

func (l *Londo) PublishNewSubject(exchange string, queue string, s *Subject) *Londo {

	e := EnrollEvent{
		Subject:  s.Subject,
		AltNames: s.AltNames,
		Targets:  s.Targets,
	}

	j, err := json.Marshal(&e)
	if err != nil {
		l.Log.Err <- err
		return l
	}

	if err := l.AMQP.Emit(
		exchange,
		queue,
		amqp.Publishing{
			ContentType: ContentType,
			Body:        j,
		}); err != nil {
		l.Log.Err <- err
		return l
	}
	l.Log.Info <- "enrolling new subject: " + e.Subject

	return l
}

func (l *Londo) PublishCollect(event CollectEvent) *Londo {

	j, _ := json.Marshal(&event)

	// TODO: remove duplication
	if err := l.AMQP.Emit(
		CollectExchange,
		CollectQueue,
		amqp.Publishing{
			ContentType: ContentType,
			Body:        j,
		}); err != nil {
		l.Log.Err <- err
		return l
	}
	l.Log.Info <- strconv.Itoa(event.CertID) + " has been queued to be collected"

	return l
}

func (l *Londo) PublishDbCommand(cmd string, s *Subject) *Londo {

	switch cmd {
	case DbAddSubjcommand:

		e := NewSubjectEvenet{
			Subject:    s.Subject,
			CSR:        s.CSR,
			PrivateKey: s.PrivateKey,
			CertID:     s.CertID,
			OrderID:    s.OrderID,
			AltNames:   s.AltNames,
			Targets:    s.Targets,
		}

		// TODO: need to do something about the rest.
		j, err := json.Marshal(&e)
		if err != nil {
			l.Log.Err <- err
			return l
		}

		if err := l.AMQP.Emit(
			DbReplyExchange,
			DbReplyQueue,
			amqp.Publishing{
				ContentType: ContentType,
				Type:        DbAddSubjcommand,
				Body:        j,
			}); err != nil {
			l.Log.Err <- err
			return l
		}

		l.Log.Info <- "letting db know that " + s.Subject + " needs to be created."

	default:
		l.Log.Err <- errors.New("unknown db command: " + cmd)
		return l
	}

	return l
}
