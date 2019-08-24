package londo

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

func (l *Londo) ConsumeEnroll() *Londo {
	go l.AMQP.Consume(EnrollQueue, func(d amqp.Delivery) (error, bool) {
		s, err := UnmarshalSubjMsg(&d)
		if err != nil {
			err = d.Reject(false)
			return err, false
		}

		key, err := GeneratePrivateKey(l.Config.CertParams.BitSize)
		if err != nil {
			err = d.Reject(false)
			return err, false
		}

		s.PrivateKey, err = EncodePKey(key)
		if err != nil {
			err = d.Reject(false)
			return err, false
		}

		csr, err := GenerateCSR(key, s.Subject, l.Config)
		if err != nil {
			err = d.Reject(false)
			return err, false
		}

		s.CSR, err = EncodeCSR(csr)
		if err != nil {
			err = d.Reject(false)
			return err, false
		}

		log.Info("requesting new certificate for " + s.Subject + " subject")
		log.Debug(s.CSR)
		log.Debug(s.PrivateKey)

		// A workaround for now; we don't need to process as fast as messages are being received.
		// It is more important not to overwhelm a remote API, and get ourselves potentially banned
		time.Sleep(1 * time.Minute)

		res, err := l.RestClient.Enroll(&s)
		if err != nil {
			d.Reject(true)
			return err, false
		}

		log.Debug("response: " + string(res.Body()))

		if err := l.RestClient.VerifyStatusCode(res, http.StatusOK); err != nil {
			d.Reject(true)
			return err, false
		}

		// TODO: need a better way to log remote errors

		var j EnrollResponse

		// A failure here will be handled by collector once periodic check is ready
		err = json.Unmarshal(res.Body(), &j)
		if err != nil {
			d.Reject(false)
			return err, false
		}

		l.PublishCollect(j.SslId)

		s.CertID = j.SslId
		s.OrderID = j.RenewID

		l.PublishDbCommand(DbAddSubjComd, &s, "")

		return nil, false
	})

	return l
}

/*
Since automated renew process involve manual approval by a human, it is much easier to revoke
old certificate and issue new one. While this complicates logic, currently, this is the best
approach.
*/
// TODO: need separate revoke consumer

func (l *Londo) ConsumeRenew() *Londo {
	go l.AMQP.Consume(RenewQueue, func(d amqp.Delivery) (error, bool) {
		s, err := UnmarshalSubjMsg(&d)
		if err != nil {
			d.Reject(false)
			return err, false
		}

		// Same as another consumer
		time.Sleep(1 * time.Minute)

		res, err := l.RestClient.Revoke(s.CertID)
		// TODO: Response result processing needs to be elsewhere
		if err != nil {
			d.Reject(true)
			return err, false
		}

		if err := l.RestClient.VerifyStatusCode(res, http.StatusNoContent); err != nil {
			d.Reject(true)
			return err, false
		}

		e := DeleteSubjEvent{
			CertID: s.CertID,
		}

		// The error should never happen, or should it?
		j, err := json.Marshal(&e)
		if err != nil {
			err = d.Reject(false)
			return err, false
		}

		if err := l.AMQP.Emit(
			DbReplyExchange,
			DbReplyQueue,
			amqp.Publishing{
				ContentType:   "application/json",
				Type:          DbDeleteSubjComd,
				CorrelationId: d.CorrelationId,
				Body:          j,
			}); err != nil {
			err = d.Reject(false)
			return err, false
		}

		log.Infof("requested deletion of %s", s.Subject)

		l.PublishNewSubject(&s)
		log.Infof("sent %s subject for new enrollment", s.Subject)

		return nil, false
	})

	return l
}

func (l *Londo) ConsumeCollect() *Londo {
	go l.AMQP.Consume(CollectQueue, func(d amqp.Delivery) (error, bool) {
		// TODO: fix code duplication
		s, err := UnmarshalSubjMsg(&d)
		if err != nil {
			return err, false
		}

		res, err := l.RestClient.Collect(s.CertID)
		if err != nil {
			err = d.Reject(true)
			return err, false
		}

		time.Sleep(1 * time.Minute)

		if err := l.RestClient.VerifyStatusCode(res, http.StatusOK); err != nil {
			d.Reject(true)
			return err, false
		}

		s.Certificate = string(res.Body())
		l.PublishDbCommand(DbUpdateSubjComd, &s, "")

		return nil, false
	})

	return l
}

func (l *Londo) ConsumeGrpcReplies(queue string, ch chan Subject, done chan struct{}) *Londo {
	go l.AMQP.Consume(queue, func(d amqp.Delivery) (error, bool) {
		var s Subject
		if err := json.Unmarshal(d.Body, &s); err != nil {
			return err, false
		}

		if s.Subject != "" {
			log.Infof("received %s", s.Subject)
		}
		ch <- s

		if d.Type == CloseChannelCmd {
			log.Debug("received stop command")
			if done != nil {
				done <- struct{}{}
			}
			return nil, true
		}

		return nil, false
	})

	return l
}

func (l *Londo) ConsumeDbRPC() *Londo {
	go l.AMQP.Consume(DbReplyQueue, func(d amqp.Delivery) (error, bool) {

		switch d.Type {
		case DbGetSubjectByTargetCmd:
			var e GetSubjectByTarget
			if err := json.Unmarshal(d.Body, &e); err != nil {
				return err, false
			}

			log.Infof("getting subjects for %s target(s)", e.Target)

			subjs, err := l.Db.FineManySubjects(e.Target)
			if err != nil {
				log.Error(err)
				return err, false
			}

			length := len(subjs)

			for i := 0; i <= length; i++ {

				if i == length {
					var s Subject
					l.PublishReplySubject(&s, d.ReplyTo, CloseChannelCmd)
					log.Info("sending close channel message...")
				} else {
					l.PublishReplySubject(&subjs[i], d.ReplyTo, "")
					log.Infof("sent %s back to %s queue", subjs[i].Subject, d.ReplyTo)
				}

			}

		case DbGetSubjectComd:
			var e GetSubjectEvenet
			if err := json.Unmarshal(d.Body, &e); err != nil {
				return err, false
			}

			subj, err := l.Db.FindSubject(e.Subject)

			l.PublishReplySubject(&subj, d.ReplyTo, CloseChannelCmd)

			if err != nil {
				log.Error(errors.New("subject " + e.Subject + " not found"))
			} else {
				log.Infof("sent %s back to %s queue", subj.Subject, d.ReplyTo)
			}

		case DbDeleteSubjComd:
			certId, err := l.deleteSubject(&d)
			if err != nil {
				return err, false
			}

			log.Infof("certificate %d has been deleted", certId)

		case DbAddSubjComd:
			subj, err := l.createNewSubject(&d)
			if err != nil {
				return err, false
			}

			log.Infof("%s has been added", subj)

		case DbUpdateSubjComd:
			certId, err := l.updateSubject(&d)
			if err != nil {
				return err, false
			}

			log.Infof("subject with %d has been updated with new certificate", certId)

		default:
			log.Warn("unknown command received: %s", d.Type)
		}

		return nil, false
	})

	return l
}

func (l *Londo) updateSubject(d *amqp.Delivery) (int, error) {
	var e CompleteEnrollEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return 0, err
	}

	c, err := ParsePublicCertificate(e.Certificate)
	if err != nil {
		return 0, err
	}

	return e.CertID, l.Db.UpdateSubjCert(e.CertID, e.Certificate, c.NotAfter)
}

func (l *Londo) createNewSubject(d *amqp.Delivery) (string, error) {
	var e NewSubjectEvenet
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return "", err
	}

	return e.Subject, l.Db.InsertSubject(&Subject{
		Subject:    e.Subject,
		CSR:        e.CSR,
		PrivateKey: e.PrivateKey,
		CertID:     e.CertID,
		OrderID:    e.OrderID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Targets:    e.Targets,
		AltNames:   e.AltNames,
	})
}

func (l *Londo) deleteSubject(d *amqp.Delivery) (int, error) {
	var e DeleteSubjEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return 0, err
	}

	return e.CertID, l.Db.DeleteSubject(d.CorrelationId, e.CertID)
}
