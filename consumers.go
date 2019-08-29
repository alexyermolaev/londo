package londo

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

func (l *Londo) ConsumeEnroll() *Londo {
	go l.AMQP.Consume(EnrollQueue, nil, func(d amqp.Delivery) (error, bool) {
		s, err := UnmarshalSubjMsg(&d)
		if err != nil {
			err = d.Reject(false)
			return err, false
		}

		key, err := GeneratePrivateKey(cfg.CertParams.BitSize)
		if err != nil {
			err = d.Reject(false)
			return err, false
		}

		s.PrivateKey, err = EncodePKey(key)
		if err != nil {
			err = d.Reject(false)
			return err, false
		}

		csr, err := GenerateCSR(key, s.Subject, cfg)
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

		if err := l.Publish(CollectExchange, CollectQueue, CollectEvent{CertID: j.SslId}, "", ""); err != nil {
			return nil, false
		}
		log.Info("sub %d -> collect", j.SslId)

		s.CertID = j.SslId
		s.OrderID = j.RenewID

		if err := l.Publish(DbReplyExchange, DbReplyQueue, NewSubjectEvent{
			Subject:    s.Subject,
			CSR:        s.CSR,
			PrivateKey: s.PrivateKey,
			CertID:     s.CertID,
			OrderID:    s.OrderID,
			AltNames:   s.AltNames,
			Targets:    s.Targets,
		}, "", DbAddSubjCmd); err != nil {
			return err, false
		}

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
	go l.AMQP.Consume(RenewQueue, nil, func(d amqp.Delivery) (error, bool) {
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

		// TODO: this doesn't belong here
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
				Type:          DbDeleteSubjCmd,
				CorrelationId: d.CorrelationId,
				Body:          j,
			}); err != nil {
			err = d.Reject(false)
			return err, false
		}

		log.Infof("requested deletion of %s", s.Subject)

		if err := l.Publish(EnrollExchange, EnrollQueue, EnrollEvent{
			Subject:  s.Subject,
			AltNames: s.AltNames,
			Targets:  s.Targets,
		}, "", ""); err != nil {
			return nil, false
		}
		//l.PublishNewSubject(&s)
		log.Infof("sent %s subject for new enrollment", s.Subject)

		return nil, false
	})

	return l
}

func (l *Londo) ConsumeCollect() *Londo {
	go l.AMQP.Consume(CollectQueue, nil, func(d amqp.Delivery) (error, bool) {
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
		if err := l.Publish(DbReplyExchange, DbReplyQueue, CompleteEnrollEvent{
			CertID:      s.CertID,
			Certificate: s.Certificate,
		}, "", DbUpdateSubjCmd); err != nil {
			return err, false
		}

		return nil, false
	})

	return l
}

func (l *Londo) ConsumeGrpcReplies(
	queue string,
	ch chan Subject,
	done chan struct{},
	wg *sync.WaitGroup) *Londo {

	go l.AMQP.Consume(queue, wg, func(d amqp.Delivery) (error, bool) {

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

func (l *Londo) ConsumeCheck() *Londo {
	go l.AMQP.Consume(CheckQueue, nil, func(d amqp.Delivery) (error, bool) {
		var e CheckDNSEvent
		if err := json.Unmarshal(d.Body, &e); err != nil {
			return err, false
		}

		log.Infof("received %s subject to be checked", e.Subject)

		now := time.Now()
		t := e.Unresolvable.Sub(now).Round(time.Hour).Hours()

		ips, err := net.LookupIP(e.Subject)
		if err != nil {
			d.Reject(false)
			return err, false
		}

		// TODO: unhardcode this via flag, config and env
		if t > 168 && len(ips) == 0 {
			// TODO: delete/revoke
			d.Reject(false)
		}

		if len(ips) == 0 {
			e.Unresolvable = time.Now()
			if err := l.Publish(CheckExchange, CheckQueue, &e, "", ""); err != nil {
				return err, false
			}
			return nil, false
		}

		e.Targets = nil
		for _, ip := range ips {
			e.Targets = append(e.Targets, ip.String())
		}

		if err := l.Publish(CheckExchange, CheckQueue, &e, "", ""); err != nil {
			return err, false
		}

		return nil, false
	})

	return l
}

func (l *Londo) ConsumeDbRPC() *Londo {
	go l.AMQP.Consume(DbReplyQueue, nil, func(d amqp.Delivery) (error, bool) {
		switch d.Type {
		case DbUpdateUnreachSubjCmd:
			var e CheckDNSEvent
			if err := json.Unmarshal(d.Body, &e); err != nil {
				return err, false
			}

			log.Infof("sub %s: update unreach %v", e.Subject, e.Unresolvable)

			if err := l.Db.UpdateUnreachable(&e.Subject, &e.Unresolvable); err != nil {
				log.Error(err)
				return err, false
			}

		case DbGetExpiringSubjectsCmd:
			// TODO: need refactor to get rid of duplication
			var e GetExpiringSubjEvent
			if err := json.Unmarshal(d.Body, &e); err != nil {
				return err, false
			}

			log.Infof("getting subjects expiring in %d days", e.Days)

			exp, err := l.Db.FindExpiringSubjects(24 * int(e.Days))
			if err != nil {
				log.Error(err)
				return err, false
			}

			length := len(exp) - 1
			var cmd string

			if length == -1 {
				var s Subject

				if err := l.Publish(GRPCServerExchange, d.ReplyTo, &s, d.ReplyTo, CloseChannelCmd); err != nil {
					return err, false
				}
				log.Infof("sent none -> %s queue", d.ReplyTo)
				return nil, false
			}

			for i := 0; i <= length; i++ {

				if i == length {
					cmd = CloseChannelCmd
				}

				if err := l.Publish(GRPCServerExchange, d.ReplyTo, exp[i], d.ReplyTo, cmd); err != nil {
					return err, false
				}
				log.Infof("sent %s -> %s queue", exp[i].Subject, d.ReplyTo)
			}

		case DbGetSubjectByTargetCmd:
			var e GetSubjectByTargetEvent
			if err := json.Unmarshal(d.Body, &e); err != nil {
				return err, false
			}

			log.Infof("getting subjects for %s target(s)", e.Target)

			subjs, err := l.Db.FineManySubjects(e.Target)
			if err != nil {
				log.Error(err)
				return err, false
			}

			length := len(subjs) - 1
			var cmd string

			if length == -1 {
				var s Subject

				if err := l.Publish(GRPCServerExchange, d.ReplyTo, &s, d.ReplyTo, CloseChannelCmd); err != nil {
					return err, false
				}
				log.Infof("sent none -> %s queue", d.ReplyTo)
				return nil, false
			}

			for i := 0; i <= length; i++ {

				if i == length {
					cmd = CloseChannelCmd
				}

				if err := l.Publish(GRPCServerExchange, d.ReplyTo, &subjs[i], d.ReplyTo, cmd); err != nil {
					return err, false
				}
				log.Infof("sent %s -> %s queue", subjs[i].Subject, d.ReplyTo)
			}

		case DbGetSubjectCmd:
			var e GetSubjectEvent
			if err := json.Unmarshal(d.Body, &e); err != nil {
				return err, false
			}

			subj, err := l.Db.FindSubject(e.Subject)

			if err := l.Publish(GRPCServerExchange, d.ReplyTo, &subj, d.ReplyTo, CloseChannelCmd); err != nil {
				return err, false
			}

			if err != nil {
				log.Error(errors.New("subject " + e.Subject + " not found"))
			} else {
				log.Infof("sent %s back to %s queue", subj.Subject, d.ReplyTo)
			}

		case DbDeleteSubjCmd:
			certId, err := l.deleteSubject(&d)
			if err != nil {
				return err, false
			}

			log.Infof("certificate %d has been deleted", certId)

		case DbAddSubjCmd:
			subj, err := l.createNewSubject(&d)
			if err != nil {
				return err, false
			}

			log.Infof("%s has been added", subj)

		case DbUpdateSubjCmd:
			certId, err := l.updateSubject(&d)
			if err != nil {
				return err, false
			}

			log.Infof("subject with %d has been updated with new certificate", certId)

		default:
			log.Warnf("unknown command received: %s", d.Type)
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
	var e NewSubjectEvent
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
