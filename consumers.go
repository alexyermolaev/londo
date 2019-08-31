package londo

import (
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

func (l *Londo) ConsumeEnroll() *Londo {
	go l.AMQP.Consume(EnrollQueue, nil, func(d amqp.Delivery) bool {
		s, err := UnmarshalSubjMsg(&d)
		if err != nil {
			err = d.Reject(false)
			log.WithFields(logrus.Fields{logAction: "rejected"}).Error(err)
			return false
		}

		key, err := GeneratePrivateKey(cfg.CertParams.BitSize)
		if err != nil {
			err = d.Reject(false)
			log.WithFields(logrus.Fields{logAction: "rejected"}).Error(err)
			return false
		}

		s.PrivateKey, err = EncodePKey(key)
		if err != nil {
			err = d.Reject(false)
			log.WithFields(logrus.Fields{logAction: "rejected"}).Error(err)
			return false
		}

		csr, err := GenerateCSR(key, s.Subject, cfg)
		if err != nil {
			err = d.Reject(false)
			log.WithFields(logrus.Fields{logAction: "rejected"}).Error(err)
			return false
		}

		s.CSR, err = EncodeCSR(csr)
		if err != nil {
			err = d.Reject(false)
			log.WithFields(logrus.Fields{logAction: "rejected"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{logSubject: s.Subject}).Info("enrolling")

		// A workaround for now; we don't need to process as fast as messages are being received.
		// It is more important not to overwhelm a remote API, and get ourselves potentially banned
		time.Sleep(1 * time.Minute)

		res, err := l.RestClient.Enroll(&s)
		if err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logAction: "requeue"}).Error(err)
			return false
		}

		log.Debug("response: " + string(res.Body()))

		if err := l.RestClient.VerifyStatusCode(res, http.StatusOK); err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logAction: "requeue"}).Error(err)
			return false
		}

		// TODO: need a better way to log remote errors

		var j EnrollResponse

		// A failure here will be handled by collector once periodic check is ready
		err = json.Unmarshal(res.Body(), &j)
		if err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logAction: "rejected"}).Error(err)
			return false
		}

		if err := l.Publish(
			CollectExchange, CollectQueue, "", "", CollectEvent{CertID: j.SslId}); err != nil {

			log.WithFields(logrus.Fields{
				logExchange: DbReplyExchange,
				logQueue:    DbReplyQueue,
				logCertID:   s.CertID}).Error(err)

			return false
		}

		log.WithFields(logrus.Fields{
			logExchange: CollectExchange,
			logQueue:    CollectQueue,
			logCertID:   j.SslId}).Info("published")

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
				logExchange: DbReplyExchange,
				logQueue:    DbReplyQueue,
				logSubject:  s.Subject,
				logCertID:   s.CertID}).Error(err)

			return false
		}

		log.WithFields(logrus.Fields{
			logExchange: DbReplyExchange,
			logQueue:    DbReplyQueue,
			logSubject:  s.Subject,
			logCertID:   s.CertID}).Info("published")

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
			log.WithFields(logrus.Fields{logReason: err}).Error("rejected")
			return false
		}

		log.WithFields(logrus.Fields{logSubject: s.Subject}).Info("delivered")

		time.Sleep(1 * time.Minute)

		res, err := l.RestClient.Revoke(s.CertID, "renew")
		// TODO: Response result processing needs to be elsewhere
		if err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logReason: err}).Error("requeue")
			return false
		}

		if err := l.RestClient.VerifyStatusCode(res, http.StatusNoContent); err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logReason: err}).Error("requeue")
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
				logExchange: DbReplyExchange,
				logQueue:    DbReplyQueue,
				logCmd:      DbDeleteSubjCmd,
				logReason:   err,
				logSubject:  s.Subject,
				logCertID:   s.CertID}).Error("msg lost")

			return false
		}

		log.WithFields(logrus.Fields{
			logExchange: DbReplyExchange,
			logQueue:    DbReplyQueue,
			logCmd:      DbDeleteSubjCmd,
			logSubject:  s.Subject,
			logCertID:   s.CertID}).Info("published")

		if err := l.Publish(EnrollExchange, EnrollQueue, "", "", EnrollEvent{
			Subject:  s.Subject,
			Port:     s.Port,
			AltNames: s.AltNames,
			Targets:  s.Targets,
		}); err != nil {

			log.WithFields(logrus.Fields{
				logExchange: EnrollExchange,
				logQueue:    EnrollQueue,
				logReason:   err,
				logSubject:  s.Subject}).Error("msg lost")

			return false
		}

		log.WithFields(logrus.Fields{
			logExchange: EnrollExchange,
			logQueue:    EnrollQueue,
			logSubject:  s.Subject}).Info("published")

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
			log.WithFields(logrus.Fields{logAction: "rejected"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{logCertID: s.CertID}).Info("collecting")

		time.Sleep(1 * time.Minute)

		res, err := l.RestClient.Collect(s.CertID)
		if err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logAction: "requeue"}).Error(err)
			return false
		}

		if err := l.RestClient.VerifyStatusCode(res, http.StatusOK); err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logAction: "requeue"}).Error(err)
			return false
		}

		s.Certificate = string(res.Body())

		if err := l.Publish(DbReplyExchange, DbReplyQueue, "", DbUpdateSubjCmd, CompleteEnrollEvent{
			CertID:      s.CertID,
			Certificate: s.Certificate,
		}); err != nil {
			d.Reject(true)
			log.WithFields(logrus.Fields{logAction: "requeue"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{
			logExchange: DbReplyExchange,
			logQueue:    DbReplyQueue,
			logCertID:   s.CertID}).Info("published")

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
			log.WithFields(logrus.Fields{logQueue: queue, logSubject: s.Subject}).Info("consume")
		}
		ch <- s

		if d.Type == CloseChannelCmd {
			log.WithFields(logrus.Fields{logQueue: queue, logCmd: CloseChannelCmd}).Debug("received")
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
			log.WithFields(logrus.Fields{logReason: err}).Error("rejected")
			return false
		}

		log.WithFields(logrus.Fields{logSubject: e.Subject}).Info("received")

		now := time.Now().UTC()
		t := now.Sub(e.Unresolvable).Round(time.Hour).Hours()

		ips, err := net.LookupIP(e.Subject)

		// if DNS cannot resolve the host and unresolvable time is larger than set number of hours
		// but unresolvable time itself isn't a zero, revoke delete
		// TODO: unhardcode this
		if err != nil && t > 168 && !e.Unresolvable.IsZero() {
			// TODO: delete/revoke
			log.WithFields(logrus.Fields{
				logSubject: e.Subject,
				logHours:   int(t)}).Info("delete")

			d.Ack(false)
			return false
		}

		var curSerial big.Int
		curSerial.SetString(e.Serial, 10)

		e.Match = false
		e.Targets = nil
		e.Outdated = nil

		// if dns can't resolve but it previous could, because unresolvable time was reset back to zero
		if len(ips) == 0 && e.Unresolvable.IsZero() {
			e.Unresolvable = now

			log.WithFields(logrus.Fields{logSubject: e.Subject}).Info("unreachable")
		}

		// we have an array of IPs and unresolvable time is zero
		if len(ips) != 0 {
			e.Unresolvable = time.Time{}
			var match int

			for _, ip := range ips {

				serial, err := GetCertSerialNumber(ip.String(), e.Port, e.Subject)
				if err != nil {
					log.Debug(err)
				}

				if serial.Cmp(&curSerial) == 0 {
					e.Targets = append(e.Targets, ip.String())
					match++

					log.WithFields(logrus.Fields{
						logSubject: e.Subject,
						logTarget:  ip.String()}).Info("added")

				} else {
					e.Outdated = append(e.Outdated, ip.String())

					if Debug {
						log.WithFields(logrus.Fields{
							logSubject:  e.Subject,
							logSerial:   curSerial.String(),
							logDbSerial: serial.String()}).Debug("added")
					} else {
						log.WithFields(logrus.Fields{
							logSubject:  e.Subject,
							logOutdated: ip.String()}).Info("added")
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
				logSubject:  e.Subject,
				logExchange: DbReplyExchange,
				logQueue:    DbReplyQueue,
				logReason:   err,
				logCmd:      DbUpdateCertStatusCmd}).Error("rejected")

			return false
		}

		log.WithFields(logrus.Fields{
			logSubject:  e.Subject,
			logExchange: DbReplyExchange,
			logQueue:    DbReplyQueue,
			logCmd:      DbUpdateCertStatusCmd}).Info("published")

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
