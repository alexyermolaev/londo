package londo

import (
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/alexyermolaev/londo/logger"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

func (l *Londo) ConsumeEnroll() *Londo {
	go l.AMQP.Consume(EnrollQueue, nil, func(d amqp.Delivery) bool {
		s, err := UnmarshalSubjMsg(&d)
		if err != nil {
			err = d.Reject(false)
			log.WithFields(logrus.Fields{logger.Action: "rejected"}).Error(err)
			return false
		}

		key, err := GeneratePrivateKey(cfg.CertParams.BitSize)
		if err != nil {
			err = d.Reject(false)
			log.WithFields(logrus.Fields{logger.Action: "rejected"}).Error(err)
			return false
		}

		s.PrivateKey, err = EncodePKey(key)
		if err != nil {
			err = d.Reject(false)
			log.WithFields(logrus.Fields{logger.Action: "rejected"}).Error(err)
			return false
		}

		csr, err := GenerateCSR(key, s.Subject, cfg)
		if err != nil {
			err = d.Reject(false)
			log.WithFields(logrus.Fields{logger.Action: "rejected"}).Error(err)
			return false
		}

		s.CSR, err = EncodeCSR(csr)
		if err != nil {
			err = d.Reject(false)
			log.WithFields(logrus.Fields{logger.Action: "rejected"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{logger.Subject: s.Subject}).Info("enrolling")

		// A workaround for now; we don't need to process as fast as messages are being received.
		// It is more important not to overwhelm a remote API, and get ourselves potentially banned
		time.Sleep(1 * time.Minute)

		res, err := l.RestClient.Enroll(&s)
		if err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logger.Action: "requeue"}).Error(err)
			return false
		}

		log.Debug("response: " + string(res.Body()))

		if err := l.RestClient.VerifyStatusCode(res, http.StatusOK); err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logger.Action: "requeue"}).Error(err)
			return false
		}

		// TODO: need a better way to log remote errors

		var j EnrollResponse

		// A failure here will be handled by collector once periodic check is ready
		err = json.Unmarshal(res.Body(), &j)
		if err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logger.Action: "rejected"}).Error(err)
			return false
		}

		if err := l.Publish(
			CollectExchange, CollectQueue, "", "", CollectEvent{CertID: j.SslId}); err != nil {

			log.WithFields(logrus.Fields{
				logger.Exchange: DbReplyExchange,
				logger.Queue:    DbReplyQueue,
				logger.CertID:   s.CertID}).Error(err)

			return false
		}

		log.WithFields(logrus.Fields{
			logger.Exchange: CollectExchange,
			logger.Queue:    CollectQueue,
			logger.CertID:   j.SslId}).Info("published")

		s.CertID = j.SslId
		s.OrderID = j.RenewID

		if err := l.Publish(DbReplyExchange, DbReplyQueue, "", DbAddSubjCmd, NewSubjectEvent{
			Subject:    s.Subject,
			Port:       s.Port,
			CSR:        s.CSR,
			PrivateKey: s.PrivateKey,
			CertID:     s.CertID,
			OrderID:    s.OrderID,
			AltNames:   s.AltNames,
			Targets:    s.Targets,
		}); err != nil {

			log.WithFields(logrus.Fields{
				logger.Exchange: DbReplyExchange,
				logger.Queue:    DbReplyQueue,
				logger.Subject:  s.Subject,
				logger.CertID:   s.CertID}).Error(err)

			return false
		}

		log.WithFields(logrus.Fields{
			logger.Exchange: DbReplyExchange,
			logger.Queue:    DbReplyQueue,
			logger.Subject:  s.Subject,
			logger.CertID:   s.CertID}).Info("published")

		d.Ack(false)
		return false
	})

	return l
}

/*
Since automated renew process involve manual approval by a human, it is much easier to revoke
old certificate and issue new one. While this complicates logic, currently, this is the best
approach.
*/
// TODO: need a separate revoke consumer
func (l *Londo) ConsumeRenew() *Londo {
	go l.AMQP.Consume(RenewQueue, nil, func(d amqp.Delivery) bool {
		s, err := UnmarshalSubjMsg(&d)
		if err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logger.Reason: err}).Error(logger.Rejected)
			return false
		}

		log.WithFields(logrus.Fields{logger.Subject: s.Subject}).Info(logger.Received)

		time.Sleep(1 * time.Minute)

		if err := l.Publish(RevokeExchange, RevokeQueue, "", "", RevokeEvent{
			ID:     s.ID.Hex(),
			CertID: s.CertID,
		}); err != nil {
			d.Reject(false)

			log.WithFields(logrus.Fields{
				logger.Exchange: RevokeExchange,
				logger.Queue:    RevokeQueue,
				logger.CertID:   s.CertID,
				logger.Reason:   err,
			}).Error(logger.Rejected)

			return false
		}

		d.Ack(false)

		if err := l.Publish(
			DbReplyExchange,
			DbReplyQueue,
			"",
			DbDeleteSubjCmd,
			RevokeEvent{CertID: s.CertID, ID: d.CorrelationId},
		); err != nil {

			log.WithFields(logrus.Fields{
				logger.Exchange: DbReplyExchange,
				logger.Queue:    DbReplyQueue,
				logger.Cmd:      DbDeleteSubjCmd,
				logger.Reason:   err,
				logger.Subject:  s.Subject,
				logger.CertID:   s.CertID}).Error("msg lost")

			return false
		}

		log.WithFields(logrus.Fields{
			logger.Exchange: DbReplyExchange,
			logger.Queue:    DbReplyQueue,
			logger.Cmd:      DbDeleteSubjCmd,
			logger.Subject:  s.Subject,
			logger.CertID:   s.CertID}).Info(logger.Published)

		if err := l.Publish(EnrollExchange, EnrollQueue, "", "", EnrollEvent{
			Subject:  s.Subject,
			Port:     s.Port,
			AltNames: s.AltNames,
			Targets:  s.Targets,
		}); err != nil {

			log.WithFields(logrus.Fields{
				logger.Exchange: EnrollExchange,
				logger.Queue:    EnrollQueue,
				logger.Reason:   err,
				logger.Subject:  s.Subject}).Error("msg lost")

			return false
		}

		log.WithFields(logrus.Fields{
			logger.Exchange: EnrollExchange,
			logger.Queue:    EnrollQueue,
			logger.Subject:  s.Subject}).Info(logger.Published)

		return false
	})

	return l
}

func (l *Londo) ConsumeRevoke() *Londo {
	go l.AMQP.Consume(RevokeQueue, nil, func(d amqp.Delivery) bool {

		var e RevokeEvent
		if err := json.Unmarshal(d.Body, &e); err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logger.Reason: err}).Error("rejected")
			return false
		}

		log.WithFields(logrus.Fields{logger.CertID: e.CertID}).Info(logger.Received)

		time.Sleep(1 * time.Minute)

		res, err := l.RestClient.Revoke(s.CertID, "renew")
		if err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logger.Reason: err}).Error(logger.Requeue)
			return false
		}

		if err := l.RestClient.VerifyStatusCode(res, http.StatusNoContent); err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logger.Reason: err}).Error(logger.Requeue)
			return false
		}

		log.WithFields(logrus.Fields{logger.CertID: e.CertID}).Info(logger.Revoked)

		d.Ack(false)
		return false
	})

	return l
}

func (l *Londo) ConsumeCollect() *Londo {
	go l.AMQP.Consume(CollectQueue, nil, func(d amqp.Delivery) bool {
		// TODO: fix code duplication
		s, err := UnmarshalSubjMsg(&d)
		if err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logger.Action: "rejected"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{logger.CertID: s.CertID}).Info("collecting")

		time.Sleep(1 * time.Minute)

		res, err := l.RestClient.Collect(s.CertID)
		if err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logger.Action: "requeue"}).Error(err)
			return false
		}

		if err := l.RestClient.VerifyStatusCode(res, http.StatusOK); err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logger.Action: "requeue"}).Error(err)
			return false
		}

		s.Certificate = string(res.Body())

		if err := l.Publish(DbReplyExchange, DbReplyQueue, "", DbUpdateSubjCmd, CompleteEnrollEvent{
			CertID:      s.CertID,
			Certificate: s.Certificate,
		}); err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logger.Action: "requeue"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{
			logger.Exchange: DbReplyExchange,
			logger.Queue:    DbReplyQueue,
			logger.CertID:   s.CertID}).Info("published")

		d.Ack(false)
		return false
	})

	return l
}

func (l *Londo) ConsumeGRPCReplies(
	queue string,
	ch chan Subject,
	done chan struct{},
	wg *sync.WaitGroup) *Londo {

	go l.AMQP.Consume(queue, wg, func(d amqp.Delivery) bool {

		var s Subject
		if err := json.Unmarshal(d.Body, &s); err != nil {
			return false
		}

		if s.Subject != "" {
			log.WithFields(logrus.Fields{logger.Queue: queue, logger.Subject: s.Subject}).Info("consume")
		}
		ch <- s

		if d.Type == CloseChannelCmd {
			log.WithFields(logrus.Fields{logger.Queue: queue, logger.Cmd: CloseChannelCmd}).Debug("received")
			if done != nil {
				done <- struct{}{}
			}
			return true
		}

		d.Ack(false)
		return false
	})

	return l
}

func (l *Londo) ConsumeCheck() *Londo {
	go l.AMQP.Consume(CheckQueue, nil, func(d amqp.Delivery) bool {
		var e CheckCertEvent

		if err := json.Unmarshal(d.Body, &e); err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logger.Reason: err}).Error(logger.Rejected)
			return false
		}

		log.WithFields(logrus.Fields{logger.Subject: e.Subject}).Info(logger.Received)

		now := time.Now().UTC()
		t := now.Sub(e.Unresolvable).Round(time.Hour).Hours()

		ips, err := net.LookupIP(e.Subject)

		// if DNS cannot resolve the host and unresolvable time is larger than set number of hours
		// but unresolvable time itself isn't a zero, revoke delete
		if err != nil && t > float64(RevokeHours) && !e.Unresolvable.IsZero() {
			revoke := RevokeEvent{
				ID:     e.ID,
				CertID: e.CertID,
			}

			if err := l.Publish(
				DbReplyExchange, DbReplyQueue, "", DbDeleteSubjCmd, revoke); err != nil {
				d.Reject(false)

				log.WithFields(logrus.Fields{
					logger.Exchange: DbReplyExchange,
					logger.Queue:    DbReplyQueue,
					logger.Reason:   err,
				}).Error(logger.Rejected)

				return false
			}
			log.WithFields(logrus.Fields{
				logger.Exchange: DbReplyExchange,
				logger.Queue:    DbReplyQueue,
				logger.Cmd:      DbDeleteSubjCmd,
				logger.Subject:  e.Subject,
				logger.Hours:    int(t)}).Info(logger.Published)

			if err := l.Publish(RevokeExchange, RevokeQueue, "", "", revoke); err != nil {
				// It isn't that important if we can't publish to revoker somehow.
				// So we'll just continue with the rest.
				log.WithFields(logrus.Fields{
					logger.Exchange: RevokeExchange,
					logger.Queue:    RevokeQueue,
					logger.Reason:   err,
				}).Error(logger.Skip)
			} else {
				log.WithFields(logrus.Fields{
					logger.Exchange: RevokeExchange,
					logger.Queue:    RevokeQueue,
					logger.Subject:  e.Subject,
					logger.CertID:   e.CertID,
					logger.Hours:    int(t)}).Info(logger.Published)
			}

			d.Ack(false)
			return false
		}

		var curSerial big.Int
		curSerial.SetString(e.Serial, 10)

		e.Match = false
		e.Targets = nil
		e.Outdated = nil
		port := strconv.Itoa(int(e.Port))

		// if dns can't resolve but it previous could, because unresolvable time was reset back to zero
		if len(ips) == 0 && e.Unresolvable.IsZero() {
			e.Unresolvable = now

			log.WithFields(logrus.Fields{logger.Subject: e.Subject}).Warn("unreachable")
		}

		// we have an array of IPs and unresolvable time is zero
		if len(ips) != 0 {
			e.Unresolvable = time.Time{}
			var match int

			for _, ip := range ips {

				i := ip.String()

				serial, err := GetCertSerialNumber(i, port, e.Subject)
				if err != nil {

					log.WithFields(logrus.Fields{
						logger.Subject: e.Subject,
						logger.IP:      i,
						logger.Port:    port,
						logger.Reason:  err,
					}).Error("skipping")

					continue
				}

				if serial.Cmp(&curSerial) == 0 {
					e.Targets = append(e.Targets, i)
					match++

					log.WithFields(logrus.Fields{
						logger.Subject: e.Subject,
						logger.Target:  i}).Info("added")

				} else {
					e.Outdated = append(e.Outdated, ip.String())

					if Debug {
						log.WithFields(logrus.Fields{
							logger.Subject:  e.Subject,
							logger.Outdated: i,
							logger.Serial:   curSerial.String(),
							logger.DbSerial: serial.String()}).Debug("added")
					} else {
						log.WithFields(logrus.Fields{
							logger.Subject:  e.Subject,
							logger.Outdated: i}).Info("added")
					}
				}
			}

			if len(ips) == match {
				e.Match = true
			}
		}

		if err := l.Publish(DbReplyExchange, DbReplyQueue, "", DbUpdateCertStatusCmd, &e); err != nil {
			d.Reject(false)

			log.WithFields(logrus.Fields{
				logger.Subject:  e.Subject,
				logger.Exchange: DbReplyExchange,
				logger.Queue:    DbReplyQueue,
				logger.Reason:   err,
				logger.Cmd:      DbUpdateCertStatusCmd}).Error("rejected")

			return false
		}

		log.WithFields(logrus.Fields{
			logger.Subject:  e.Subject,
			logger.Exchange: DbReplyExchange,
			logger.Queue:    DbReplyQueue,
			logger.Cmd:      DbUpdateCertStatusCmd}).Info("published")

		d.Ack(false)
		return false
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

	return e.CertID, l.Db.UpdateSubjCert(&e.CertID, &e.Certificate, &c.NotAfter, c.SerialNumber)
}

func (l *Londo) createNewSubject(d *amqp.Delivery) (string, error) {
	var e NewSubjectEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return "", err
	}

	return e.Subject, l.Db.InsertSubject(&Subject{
		Subject:    e.Subject,
		CSR:        e.CSR,
		Port:       e.Port,
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
	var e RevokeEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return 0, err
	}

	return e.CertID, l.Db.DeleteSubject(e.ID, e.CertID)
}
