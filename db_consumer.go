package londo

import (
	"encoding/json"
	"errors"

	"github.com/alexyermolaev/londo/logger"
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

		case DbGetAllSubjectsCmd:
			return l.dbGetAllSubjects(d)

		default:
			d.Reject(false)
			log.WithFields(logrus.Fields{logger.Cmd: d.Type}).Error("unknown")
		}

		return false
	})

	return l
}

func (l *Londo) dbGetAllSubjects(d amqp.Delivery) bool {
	subjs, err := l.Db.FindAllSubjects()
	if err != nil {
		d.Reject(false)
		log.WithFields(logrus.Fields{logger.Action: "reject"}).Error(err)
		return false
	}

	d.Ack(false)

	var count int

	for _, s := range subjs {

		if err := l.Publish(CheckExchange, CheckQueue, "", "", CheckCertEvent{
			ID:           s.ID.Hex(),
			Subject:      s.Subject,
			CertID:       s.CertID,
			Serial:       s.Serial,
			Port:         s.Port,
			Match:        s.Match,
			Targets:      s.Targets,
			Outdated:     s.Outdated,
			Unresolvable: s.UnresolvableAt,
		}); err != nil {
			log.WithFields(logrus.Fields{logger.Action: "no_pub"}).Error(err)
			continue
		}

		log.WithFields(logrus.Fields{
			logger.Exchange: CheckExchange,
			logger.Queue:    CheckQueue,
			logger.Subject:  s.Subject}).Debug("published")

		count++
	}

	log.WithFields(logrus.Fields{
		logger.Exchange: CheckExchange,
		logger.Queue:    CheckQueue,
		logger.Count:    count,
		logger.Action:   "published"}).Info("published")

	return false
}

func (l *Londo) dbUpdateSubject(d amqp.Delivery) bool {
	certId, err := l.updateSubject(&d)
	if err != nil {
		d.Reject(true)
		log.WithFields(logrus.Fields{logger.Action: "requeue"}).Error(err)
		return false
	}

	log.WithFields(logrus.Fields{logger.CertID: certId, logger.Cmd: DbUpdateSubjCmd}).Info("success")
	d.Ack(false)
	return false
}

func (l *Londo) dbAddSubject(d amqp.Delivery) bool {
	subj, err := l.createNewSubject(&d)
	if err != nil {
		d.Reject(true)
		log.WithFields(logrus.Fields{logger.Action: "requeue"}).Error(err)
		return false
	}

	log.WithFields(logrus.Fields{logger.Subject: subj, logger.Cmd: DbAddSubjCmd}).Info("success")
	d.Ack(false)
	return false
}

func (l *Londo) dbDeleteSubject(d amqp.Delivery) bool {
	certId, err := l.deleteSubject(&d)
	if err != nil {
		d.Reject(true)
		log.WithFields(logrus.Fields{logger.Reason: err}).Error(logger.Requeue)
		return false
	}

	log.WithFields(logrus.Fields{logger.CertID: certId, logger.Cmd: DbDeleteSubjCmd}).Info(logger.Success)
	d.Ack(false)
	return false
}

func (l *Londo) dbGetSubjects(d amqp.Delivery) bool {
	var e GetSubjectEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		// TODO: need to reply back to sender with an error
		d.Reject(false)
		log.WithFields(logrus.Fields{logger.Action: "reject"}).Error(err)
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
			logger.Queue:   d.ReplyTo,
			logger.Subject: subj.Subject,
			logger.Cmd:     DbGetSubjectCmd}).Info("published")
	}

	d.Ack(false)
	return false
}

func (l *Londo) dbSubjectByTarget(d amqp.Delivery) bool {
	var e GetSubjectByTargetEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		d.Reject(false)
		log.WithFields(logrus.Fields{logger.Action: "reject"}).Error(err)
		return false
	}

	log.WithFields(logrus.Fields{
		logger.Cmd: DbGetSubjectByTargetCmd, logger.Targets: e.Target}).Info("get")

	subjs, err := l.Db.FineManySubjects(e.Target)
	if err != nil {
		d.Reject(false)
		log.WithFields(logrus.Fields{logger.Action: "reject"}).Error(err)
		return false
	}

	length := len(subjs) - 1
	var cmd string
	if length == -1 {
		var s Subject

		if err := l.Publish(
			GRPCServerExchange, d.ReplyTo, d.ReplyTo, CloseChannelCmd, &s); err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logger.Action: "reject"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{
			logger.Queue: d.ReplyTo, logger.Cmd: DbGetSubjectByTargetCmd}).Error("none")

		return false
	}

	for i := 0; i <= length; i++ {

		if i == length {
			cmd = CloseChannelCmd
		}

		if err := l.Publish(
			GRPCServerExchange, d.ReplyTo, d.ReplyTo, cmd, &subjs[i]); err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logger.Action: "reject"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{
			logger.Queue:   d.ReplyTo,
			logger.Cmd:     DbGetSubjectByTargetCmd,
			logger.Subject: subjs[i].Subject}).Info("published")
	}

	d.Ack(false)
	return false
}

func (l *Londo) dbExpiringSubjects(d amqp.Delivery) bool {
	var e GetExpiringSubjEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return false
	}

	log.WithFields(logrus.Fields{
		logger.Days: e.Days, logger.Cmd: DbGetExpiringSubjectsCmd}).Info("consumed")

	exp, err := l.Db.FindExpiringSubjects(24 * int(e.Days))
	if err != nil {
		d.Reject(false)
		log.WithFields(logrus.Fields{logger.Action: "reject"}).Error(err)
		return false
	}

	length := len(exp) - 1
	var cmd string

	if length == -1 {
		var s Subject

		if err := l.Publish(
			GRPCServerExchange, d.ReplyTo, d.ReplyTo, CloseChannelCmd, &s); err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logger.Action: "reject"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{
			logger.Queue: d.ReplyTo,
			logger.Cmd:   DbGetExpiringSubjectsCmd}).Error("none")

		return false
	}

	for i := 0; i <= length; i++ {

		if i == length {
			cmd = CloseChannelCmd
		}

		if err := l.Publish(
			GRPCServerExchange, d.ReplyTo, d.ReplyTo, cmd, exp[i]); err != nil {
			d.Reject(false)
			log.WithFields(logrus.Fields{logger.Action: "reject"}).Error(err)
			return false
		}

		log.WithFields(logrus.Fields{
			logger.Queue:   d.ReplyTo,
			logger.Subject: exp[i].Subject,
			logger.Cmd:     DbGetExpiringSubjectsCmd}).Info("published")
	}

	d.Ack(false)
	return false
}

func (l *Londo) dbStatusUpdate(d amqp.Delivery) bool {
	var e CheckCertEvent
	if err := json.Unmarshal(d.Body, &e); err != nil {
		return false
	}

	log.WithFields(logrus.Fields{
		logger.Subject: e.Subject, logger.Cmd: DbUpdateCertStatusCmd}).Info("consumed")

	if err := l.Db.UpdateUnreachable(&e); err != nil {
		d.Reject(false)
		log.WithFields(logrus.Fields{logger.Action: "reject"}).Error(err)
		return false
	}

	d.Ack(false)
	return false
}
