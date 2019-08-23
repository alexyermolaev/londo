package londo

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"time"

	"github.com/streadway/amqp"
)

func (l *Londo) ConsumeEnroll() *Londo {
	go l.AMQP.Consume(EnrollQueue, func(d amqp.Delivery) error {

		// A workaround for now; we don't need to process as fast as messages are being received.
		// It is more important not to overwhelm a remote API, and get ourselves potentially banned
		time.Sleep(1 * time.Minute)

		s, err := UnmarshallMsg(&d)
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

		logrus.Info("requesting new certificate for " + s.Subject + " subject")
		logrus.Debug(s.CSR)
		logrus.Debug(s.PrivateKey)

		res, err := l.RestClient.Enroll(&s)
		if err != nil {
			d.Reject(true)
			return err
		}

		logrus.Debug("response: " + string(res.Body()))

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
func (l *Londo) ConsumeRenew() *Londo {
	go l.AMQP.Consume(RenewQueue, func(d amqp.Delivery) error {

		// Same as another consumer
		time.Sleep(1 * time.Minute)

		s, err := UnmarshallMsg(&d)
		if err != nil {
			return err
		}

		res, err := l.RestClient.Revoke(s.CertID)
		// TODO: Response result processing needs to be elsewhere
		if err != nil {
			err = d.Reject(true)
			return err
		}

		if err := l.RestClient.VerifyStatusCode(res, http.StatusNoContent); err != nil {
			d.Reject(true)
			return err
		}

		if d.ReplyTo != "" {
			// TODO: Needs to be extract into its own method
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
				"",
				d.ReplyTo,
				amqp.Publishing{
					ContentType:   "application/json",
					Type:          DbDeleteSubjCommand,
					CorrelationId: d.CorrelationId,
					Body:          j,
				}); err != nil {
				err = d.Reject(false)
				return err
			} else {
				l.Log.Info <- "requesting deletion of " + s.Subject
			}
		}

		l.PublishNewSubject(EnrollExchange, EnrollQueue, &s)

		l.Log.Info <- "subject " + s.Subject + " received"

		return nil
	})

	return l
}

func (l *Londo) ConsumeCollect() *Londo {
	go l.AMQP.Consume(CollectQueue, func(d amqp.Delivery) error {

		time.Sleep(1 * time.Minute)

		// TODO: fix code duplication
		s, err := UnmarshallMsg(&d)
		if err != nil {
			return err
		}

		res, err := l.RestClient.Collect(s.CertID)
		if err != nil {
			err = d.Reject(true)
			return err
		}

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
			var e DeleteSubjEvent
			if err := json.Unmarshal(d.Body, &e); err != nil {
				return err
			}

			if err := l.Db.DeleteSubject(d.CorrelationId, e.CertID); err != nil {
				return err
			}

			l.Log.Info <- "certificate " + strconv.Itoa(e.CertID) + " has been deleted."

		case DbAddSubjCommand:
			// TODO: Get rid of duplication
			var e NewSubjectEvenet
			if err := json.Unmarshal(d.Body, &e); err != nil {
				return err
			}

			if err := l.Db.InsertSubject(&Subject{
				Subject:    e.Subject,
				CSR:        e.CSR,
				PrivateKey: e.PrivateKey,
				CertID:     e.CertID,
				OrderID:    e.OrderID,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				Targets:    e.Targets,
				AltNames:   e.AltNames,
			}); err != nil {
				return err
			}

		case DbUpdateSubjCommand:
			var e CompleteEnrollEvent
			if err := json.Unmarshal(d.Body, &e); err != nil {
				return err
			}

			if err := l.Db.UpdateSubjCert(e.CertID, e.Certificate); err != nil {
				return err
			}

		default:
			l.Log.Warn <- "unknown command received"
		}

		return nil
	})

	return l
}
