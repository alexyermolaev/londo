package londo

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/roylee0704/gron"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

func (l *Londo) PublishExpiringCerts() *Londo {
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
				log.Error(err)
				return
			}

			if err = l.AMQP.Emit(
				RenewExchange,
				RenewQueue,
				amqp.Publishing{
					ContentType:   ContentType,
					CorrelationId: e.ID.Hex(),
					Expiration:    strconv.Itoa(int(time.Now().Add(1 * time.Minute).Unix())),
					Body:          j,
				}); err != nil {
				log.Error(err)
				return
			}

			log.Infof("published %s", e.Subject)
		}
	})

	cron.Start()

	return l
}

func (l *Londo) PublishNewSubject(s *Subject) *Londo {

	e := EnrollEvent{
		Subject:  s.Subject,
		AltNames: s.AltNames,
		Targets:  s.Targets,
	}

	j, err := json.Marshal(&e)
	if err != nil {
		log.Error(err)
		return l
	}

	if err := l.AMQP.Emit(
		EnrollExchange,
		EnrollQueue,
		amqp.Publishing{
			ContentType: ContentType,
			Body:        j,
		}); err != nil {
		log.Error(err)
		return l
	}

	log.Infof("enrolling %s subject", e.Subject)

	return l
}

func (l *Londo) PublishCollect(event CollectEvent) *Londo {

	j, _ := json.Marshal(&event)

	if err := l.AMQP.Emit(
		CollectExchange,
		CollectQueue,
		amqp.Publishing{
			ContentType: ContentType,
			Body:        j,
		}); err != nil {
		log.Error(err)
		return l
	}

	log.Infof("%d has been queued up to be collected", event.CertID)

	return l
}

func (l *Londo) PublishDbCommand(cmd string, s *Subject) *Londo {

	var logMsg string
	var e interface{}

	switch cmd {
	case DbAddSubjCommand:
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

	case DbUpdateSubjCommand:
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
			Body:        j,
		}); err != nil {
		log.Error(err)
		return l
	}

	log.Info(logMsg)

	return l
}
