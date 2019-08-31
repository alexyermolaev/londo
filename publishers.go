package londo

import (
	"encoding/json"
	"github.com/roylee0704/gron"
	"github.com/sirupsen/logrus"
	"time"
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

// TODO: needs to be improved for resuability
func (l *Londo) PublishPeriodicly(hours int) *Londo {
	c := gron.New()

	c.AddFunc(gron.Every(time.Duration(hours)*time.Minute), func() {
		if err := l.Publish(
			DbReplyExchange,
			DbReplyQueue,
			CheckQueue,
			DbGetAllSubjectsCmd,
			EmptyEvent{},
		); err != nil {
			log.WithFields(logrus.Fields{
				logExchange: DbReplyExchange,
				logQueue:    DbReplyQueue,
				logSubject:  "all",
			}).Error(err)
		}

		log.WithFields(logrus.Fields{
			logExchange: DbReplyExchange,
			logQueue:    DbReplyQueue,
			logSubject:  "all",
		}).Info("published")
	})

	log.WithFields(logrus.Fields{
		logHours:    hours,
		logService:  "publishing",
		logQueue:    CheckQueue,
		logExchange: CheckExchange}).Info("scheduled")

	c.Start()

	return l
}
