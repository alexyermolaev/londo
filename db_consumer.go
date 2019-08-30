package londo

import (
	"encoding/json"
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

func (l *Londo) ConsumeDbRPC() *Londo {
	go l.AMQP.Consume(DbReplyQueue, nil, func(d amqp.Delivery) bool {

		switch d.Type {
		case DbUpdateCertStatusCmd:
			return l.dbStatusUpdate(d)

		case DbGetExpiringSubjectsCmd:
			return l.dbExpiringSubjects(d)

		case DbGetSubjectByTargetCmd:
			return l.dbSubjectByTarget(d)

		case DbGetSubjectCmd:
			return l.dbGetSubjects(d)

		case DbDeleteSubjCmd:
			return l.dbDeleteSubject(d)

		case DbAddSubjCmd:
			return l.dbAddSubject(d)

		case DbUpdateSubjCmd:
			return l.dbUpdateSubject(d)

		default:
			d.Reject(false)
			log.WithFields(logrus.Fields{logCmd: d.Type}).Error("unknown")
		}

		return false
	})

	return l
}

func (l *Londo) dbUpdateSubject(d amqp.Delivery) bool {
	certId, err := l.updateSubject(&d)
	if err != nil {
		d.Reject(true)
		log.WithFields(logrus.Fields{logAction: "requeue"}).Error(err)
		return false
	}

	log.WithFields(logrus.Fields{logCertID: certId, logCmd: DbUpdateSubjCmd}).Info("success")
	return false
}

func (l *Londo) dbAddSubject(d amqp.Delivery) bool {
	subj, err := l.createNewSubject(&d)
	if err != nil {
		d.Reject(true)
		log.WithFields(logrus.Fields{logAction: "requeue"}).Error(err)
		return false
	}

	log.WithFields(logrus.Fields{logSubject: subj, logCmd: DbAddSubjCmd}).Info("success")
	return false
}

func (l *Londo) dbDeleteSubject(d amqp.Delivery) bool {
	certId, err := l.deleteSubject(&d)
	if err != nil {
		d.Reject(true)
		log.WithFields(logrus.Fields{logAction: "requeue"}).Error(err)
		return false
	}

	log.WithFields(logrus.Fields{logCertID: certId, logCmd: DbDeleteSubjCmd}).Info("success")
	return false
}

func (l *Londo) dbGetSubjects(d amqp.Delivery) bool {
	var e GetSubjectEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		// TODO: need to reply back to sender with an error
		d.Reject(false)
		log.WithFields(logrus.Fields{logAction: "reject"}).Error(err)
		return false
	}

	subj, err := l.Db.FindSubject(e.Subject)
	if err := l.Publish(
		GRPCServerExchange, d.ReplyTo, d.ReplyTo, CloseChannelCmd, &subj); err != nil {
		return false
	}

	if err != nil {
		// FIXME: ????
		log.Error(errors.New("subject " + e.Subject + " not found"))
	} else {
		log.WithFields(logrus.Fields{
			logQueue:   d.ReplyTo,
			logSubject: subj.Subject,
			logCmd:     DbGetSubjectCmd}).Info("published")
	}

	return false
}

func (l *Londo) dbSubjectByTarget(d amqp.Delivery) bool {
	var e GetSubjectByTargetEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		d.Reject(false)
		log.WithFields(logrus.Fields{logAction: "reject"}).Error(err)
		return false
	}

	log.WithFields(logrus.Fields{
		logCmd: DbGetSubjectByTargetCmd, logTargets: e.Target}).Info("get")

	subjs, err := l.Db.FineManySubjects(e.Target)
	if err != nil {
		d.Reject(false)
		log.WithFields(logrus.Fields{logAction: "reject"}).Error(err)
		return false
	}

	length := len(subjs) - 1
	var cmd string
	if length == -1 {
		var s Subject

		if err := l.Publish(
			GRPCServerExchange, d.ReplyTo, d.ReplyTo, CloseChannelCmd, &s); err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logAction: "reject"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{
			logQueue: d.ReplyTo, logCmd: DbGetSubjectByTargetCmd}).Error("none")

		return false
	}

	for i := 0; i <= length; i++ {

		if i == length {
			cmd = CloseChannelCmd
		}

		if err := l.Publish(
			GRPCServerExchange, d.ReplyTo, d.ReplyTo, cmd, &subjs[i]); err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logAction: "reject"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{
			logQueue:   d.ReplyTo,
			logCmd:     DbGetSubjectByTargetCmd,
			logSubject: subjs[i].Subject}).Info("published")
	}

	return false
}

func (l *Londo) dbExpiringSubjects(d amqp.Delivery) bool {
	var e GetExpiringSubjEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return false
	}

	log.WithFields(logrus.Fields{
		logDays: e.Days, logCmd: DbGetExpiringSubjectsCmd}).Info("consumed")

	exp, err := l.Db.FindExpiringSubjects(24 * int(e.Days))
	if err != nil {
		d.Reject(false)
		log.WithFields(logrus.Fields{logAction: "reject"}).Error(err)
		return false
	}

	length := len(exp) - 1
	var cmd string

	if length == -1 {
		var s Subject

		if err := l.Publish(
			GRPCServerExchange, d.ReplyTo, d.ReplyTo, CloseChannelCmd, &s); err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logAction: "reject"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{
			logQueue: d.ReplyTo,
			logCmd:   DbGetExpiringSubjectsCmd}).Error("none")

		return false
	}

	for i := 0; i <= length; i++ {

		if i == length {
			cmd = CloseChannelCmd
		}

		if err := l.Publish(
			GRPCServerExchange, d.ReplyTo, d.ReplyTo, cmd, exp[i]); err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logAction: "reject"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{
			logQueue:   d.ReplyTo,
			logSubject: exp[i].Subject,
			logCmd:     DbGetExpiringSubjectsCmd}).Info("published")
	}

	return false
}

func (l *Londo) dbStatusUpdate(d amqp.Delivery) bool {
	var e CheckCertEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return false
	}

	log.WithFields(logrus.Fields{
		logSubject: e.Subject, logCmd: DbUpdateCertStatusCmd}).Info("consumed")

	if err := l.Db.UpdateUnreachable(&e.Subject, &e.Unresolvable, &e.NoMatch); err != nil {
		d.Reject(false)
		log.WithFields(logrus.Fields{logAction: "reject"}).Error(err)
		return false
	}

	return false
}
