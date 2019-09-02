package londo

import (
	"encoding/json"
	"time"

	"github.com/alexyermolaev/londo/logger"
	"github.com/roylee0704/gron"
	"github.com/sirupsen/logrus"
)

func (l *Londo) Publish(exchange string, queue string, reply string, cmd string, e Event) error {
	msg := e.GetMessage()
	msg.ContentType = ContentType

	if reply != "" {
		msg.ReplyTo = reply
	}

	if cmd != "" {
		msg.Type = cmd
	}

	msg.Body, err = json.Marshal(&e)
	if err != nil {
		return err
	}

	if err := l.AMQP.Emit(exchange, queue, msg); err != nil {
		return err
	}

	return nil
}

// TODO: needs to be improved for reusability
func (l *Londo) PublishPeriodically(hours int) *Londo {
	c := gron.New()

	dur := time.Hour

	if Debug {
		dur = time.Minute
	}

	c.AddFunc(gron.Every(time.Duration(hours)*dur), l.publishGetAllSubjects)

	if Debug {
		log.WithFields(logrus.Fields{
			logger.Minutes:  hours,
			logger.Service:  "publishing",
			logger.Queue:    CheckQueue,
			logger.Exchange: CheckExchange}).Warn("scheduled")
	} else {
		log.WithFields(logrus.Fields{
			logger.Hours:    hours,
			logger.Service:  "publishing",
			logger.Queue:    CheckQueue,
			logger.Exchange: CheckExchange}).Info("scheduled")
	}

	c.Start()

	return l
}

func (l *Londo) publishGetAllSubjects() {
	if err := l.Publish(
		DbReplyExchange,
		DbReplyQueue,
		CheckQueue,
		DbGetAllSubjectsCmd,
		EmptyEvent{},
	); err != nil {
		log.WithFields(logrus.Fields{
			logger.Exchange: DbReplyExchange,
			logger.Queue:    DbReplyQueue,
			logger.Subject:  "all",
		}).Error(err)
	}

	log.WithFields(logrus.Fields{
		logger.Exchange: DbReplyExchange,
		logger.Queue:    DbReplyQueue,
		logger.Subject:  "all",
	}).Info("published")
}

func (l *Londo) PublishGetAllSubjects() *Londo {
	l.publishGetAllSubjects()
	return l
}
