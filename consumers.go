package londo

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/streadway/amqp"
)

func (l *Londo) ConsumeEnroll(queue string) *Londo {
	go l.AMQP.Consume(queue, func(d amqp.Delivery) error {

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

		csr, err := GenerateCSR(key, "", l.Config)
		if err != nil {
			err = d.Reject(false)
			return err
		}

		s.CSR, err = EncodeCSR(csr)
		if err != nil {
			err = d.Reject(false)
			return err
		}

		// TODO: EncodeCSR request json, make request, process response, send message
		l.LogChannel.Info <- s.Subject
		l.LogChannel.Info <- s.CSR
		l.LogChannel.Info <- s.PrivateKey

		return nil
	})

	return l
}

/*
Since automated renew process involve manual approval by a human, it is much easier to revoke
old certificate and issue new one. While this complicates logic, currently, this is the best
approach.
*/
func (l *Londo) ConsumeRenew(queue string) *Londo {
	go l.AMQP.Consume(queue, func(d amqp.Delivery) error {

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

		if res.StatusCode() != http.StatusNoContent {
			err = d.Reject(true)
			return errors.New("remote returned " + strconv.Itoa(res.StatusCode()) + " status code")
		}

		if d.ReplyTo != "" {
			// TODO: Needs to be extract into its own method
			e := DeleteSubjEvenet{
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
					Type:          DbDeleteCommand,
					CorrelationId: d.CorrelationId,
					Body:          j,
				}); err != nil {
				err = d.Reject(false)
				return err
			} else {
				l.LogChannel.Info <- "requesting deletion of " + s.Subject
			}
		}

		l.PublishNewSubject(EnrollExchange, EnrollQueue, &s)

		l.LogChannel.Info <- "subject " + s.Subject + " received"
		return nil
	})

	return l
}

func (l *Londo) ConsumeDbRPC(queue string) *Londo {
	go l.AMQP.Consume(queue, func(d amqp.Delivery) error {

		switch d.Type {
		case DbDeleteCommand:

			var e DeleteSubjEvenet
			if err := json.Unmarshal(d.Body, &e); err != nil {
				d.Reject(false)
				return err
			}

			if err := l.Db.DeleteSubject(d.CorrelationId, e.CertID); err != nil {
				d.Reject(false)
				return err
			}

			l.LogChannel.Info <- "certificate " + strconv.Itoa(e.CertID) + " has been deleted."

		default:
			l.LogChannel.Warn <- "unknown command received"
		}

		d.Ack(false)
		return nil
	})

	return l
}
