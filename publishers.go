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

	cron.AddFunc(gron.Every(1*time.Hour), func() {
		exp, err := l.Db.FindExpiringSubjects(720)
		fail(err)

		for _, e := range exp {
			l.PublishRenew(e)
		}
	})

	cron.Start()

	return l
}

func (l *Londo) PublishRenew(s *Subject) *Londo {
	re := RenewEvent{
		Subject:  s.Subject,
		CertID:   s.CertID,
		AltNames: s.AltNames,
		Targets:  s.Targets,
	}

	j, err := json.Marshal(&re)
	if err != nil {
		log.Error(err)
		return l
	}

	if err = l.AMQP.Emit(
		RenewExchange,
		RenewQueue,
		amqp.Publishing{
			ContentType:   ContentType,
			CorrelationId: s.ID.Hex(),
			Expiration:    strconv.Itoa(int(time.Now().Add(1 * time.Minute).Unix())),
			Body:          j,
		}); err != nil {
		log.Error(err)
		return l
	}

	log.Infof("published %s", s.Subject)

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

func (l *Londo) PublishCollect(crtId int) *Londo {
	e := CollectEvent{CertID: crtId}

	// TODO: error handling
	j, _ := json.Marshal(&e)

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

	log.Infof("%d has been queued up to be collected", e.CertID)

	return l
}

func (l *Londo) PublishReplySubject(s *Subject, reply string, cmd string) *Londo {
	j, err := json.Marshal(&s)
	if err != nil {
		log.Errorf("error: %v", err)
	}

	if err := l.AMQP.Emit(
		GRPCServerExchange,
		reply,
		amqp.Publishing{
			ContentType: ContentType,
			Type:        cmd,
			Body:        j,
		}); err != nil {
		log.Error(err)
		return l
	}

	return l
}

// TODO: db commands thing needs to be redone in a better way. need to stop hacking.
func (l *Londo) PublishDbExpEvent(days int32, reply string) *Londo {
	e := GetExpringSubjEvent{Days: days}

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
			Type:        DbGetExpiringSubjectsCmd,
			ReplyTo:     reply,
			Body:        j,
		}); err != nil {
		log.Error(err)
		return l
	}

	return l
}

func (l *Londo) PublishDbCommand(cmd string, s *Subject, reply string) *Londo {
	var logMsg string
	var e interface{}

	switch cmd {
	case DbGetSubjectByTargetCmd:
		e = GetSubjectByTarget{Target: s.Targets}

	case DbGetSubjectComd:
		e = GetSubjectEvenet{Subject: s.Subject}

	case DbAddSubjComd:
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

	case DbUpdateSubjComd:
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
