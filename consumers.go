package londo

import (
	"encoding/json"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

func (l *Londo) ConsumeEnroll() *Londo {
	go l.AMQP.Consume(EnrollQueue, func(d amqp.Delivery) error {
		s, err := UnmarshalSubjMsg(&d)
		if err != nil {
			err = d.Reject(false)
			return err
		}

		key, err := GeneratePrivateKey(l.Config.CertParams.BitSize)
		if err != nil {
			err = d.Reject(false)
			return err
		}

		s.PrivateKey, err = EncodePKey(key)
		if err != nil {
			err = d.Reject(false)
			return err
		}

		csr, err := GenerateCSR(key, s.Subject, l.Config)
		if err != nil {
			err = d.Reject(false)
			return err
		}

		s.CSR, err = EncodeCSR(csr)
		if err != nil {
			err = d.Reject(false)
			return err
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
			return err
		}

		log.Debug("response: " + string(res.Body()))

		if err := l.RestClient.VerifyStatusCode(res, http.StatusOK); err != nil {
			d.Reject(true)
			return err
		}

		// TODO: need a better way to log remote errors

		var j EnrollResponse

		// A failure here will be handled by collector once periodic check is ready
		err = json.Unmarshal(res.Body(), &j)
		if err != nil {
			d.Reject(false)
			return err
		}

		l.PublishCollect(CollectEvent{CertID: j.SslId})

		s.CertID = j.SslId
		s.OrderID = j.RenewID

		l.PublishDbCommand(DbAddSubjCommand, &s)

		return nil
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
	go l.AMQP.Consume(RenewQueue, func(d amqp.Delivery) error {
		s, err := UnmarshalSubjMsg(&d)
		if err != nil {
			d.Reject(false)
			return err
		}

		// Same as another consumer
		time.Sleep(1 * time.Minute)

		res, err := l.RestClient.Revoke(s.CertID)
		// TODO: Response result processing needs to be elsewhere
		if err != nil {
			d.Reject(true)
			return err
		}

		if err := l.RestClient.VerifyStatusCode(res, http.StatusNoContent); err != nil {
			d.Reject(true)
			return err
		}

		e := DeleteSubjEvent{
			CertID: s.CertID,
		}

		// The error should never happen, or should it?
		j, err := json.Marshal(&e)
		if err != nil {
			err = d.Reject(false)
			return err
		}

		if err := l.AMQP.Emit(
			DbReplyExchange,
			DbReplyQueue,
			amqp.Publishing{
				ContentType:   "application/json",
				Type:          DbDeleteSubjCommand,
				CorrelationId: d.CorrelationId,
				Body:          j,
			}); err != nil {
			err = d.Reject(false)
			return err
		}

		log.Infof("requested deletion of %s", s.Subject)

		l.PublishNewSubject(&s)

		log.Infof("sent %s subject for new enrollment", s.Subject)

		return nil
	})

	return l
}

func (l *Londo) ConsumeCollect() *Londo {
	go l.AMQP.Consume(CollectQueue, func(d amqp.Delivery) error {
		// TODO: fix code duplication
		s, err := UnmarshalSubjMsg(&d)
		if err != nil {
			return err
		}

		res, err := l.RestClient.Collect(s.CertID)
		if err != nil {
			err = d.Reject(true)
			return err
		}

		time.Sleep(1 * time.Minute)

		if err := l.RestClient.VerifyStatusCode(res, http.StatusOK); err != nil {
			d.Reject(true)
			return err
		}

		s.Certificate = string(res.Body())
		l.PublishDbCommand(DbUpdateSubjCommand, &s)

		return nil
	})

	return l
}

func (l *Londo) ConsumeDbRPC() *Londo {
	go l.AMQP.Consume(DbReplyQueue, func(d amqp.Delivery) error {

		switch d.Type {
		case DbDeleteSubjCommand:
			var certId int
			if err := l.deleteSubject(&d, &certId); err != nil {
				return err
			}

			log.Infof("certificate %d has been deleted", certId)

		case DbAddSubjCommand:
			var subj *string
			if err := l.createNewSubject(&d, subj); err != nil {
				return err
			}

			log.Infof("added new subject: %s", subj)

		case DbUpdateSubjCommand:
			var certId int
			if err := l.updateSubject(&d, &certId); err != nil {
				return err
			}

			log.Infof("subject with %d has been updated with new certificate", certId)

		case DbGetSubjectCommand:

		default:
			log.Warn("unknown command received: %s", d.Type)
		}

		return nil
	})

	return l
}

func (l *Londo) updateSubject(d *amqp.Delivery, id *int) error {
	var e CompleteEnrollEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return err
	}
	id = &e.CertID

	c, err := ParsePublicCertificate(e.Certificate)
	if err != nil {
		return err
	}

	return l.Db.UpdateSubjCert(e.CertID, e.Certificate, c.NotAfter)
}

func (l *Londo) createNewSubject(d *amqp.Delivery, subj *string) error {
	var e NewSubjectEvenet
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return err
	}
	subj = &e.Subject

	return l.Db.InsertSubject(&Subject{
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

func (l *Londo) deleteSubject(d *amqp.Delivery, id *int) error {
	var e DeleteSubjEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return err
	}
	id = &e.CertID

	return l.Db.DeleteSubject(d.CorrelationId, e.CertID)
}
