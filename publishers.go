package londo

import (
	"encoding/json"
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
				l.LogChannel.Err <- err
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
				l.LogChannel.Err <- err
				return
			}
			l.LogChannel.Info <- "published " + e.Subject
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
		l.LogChannel.Err <- err
		return l
	}

	if err := l.AMQP.Emit(
		exchange,
		queue,
		amqp.Publishing{
			ContentType: ContentType,
			Body:        j,
		}); err != nil {
		l.LogChannel.Err <- err
		return l
	}
	l.LogChannel.Info <- "enrolling new subject: " + e.Subject

	return l
}

func (l *Londo) PublishCollect(exchange string, queue string, s *Subject) *Londo {

	e := CollectEvent{
		CertID: s.CertID,
	}

	j, err := json.Marshal(&e)
	if err != nil {
		l.LogChannel.Err <- err
		return l
	}

	// TODO: remove duplication
	if err := l.AMQP.Emit(
		exchange,
		queue,
		amqp.Publishing{
			ContentType: ContentType,
			Body:        j,
		}); err != nil {
		l.LogChannel.Err <- err
		return l
	}

	return l
}
