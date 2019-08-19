package main

import (
	"time"

	"github.com/alexyermolaev/londo"
	"github.com/roylee0704/gron"
	log "github.com/sirupsen/logrus"
)

func PublishExpiringCerts(db *londo.MongoDB, mq *londo.Londo) {
	exp, err := db.FindExpiringSubjects(720)
	londo.CheckFatalError(err)

	for _, e := range exp {
		log.Infof("%v, %v", e.Subject, e.NotAfter)
		if err := mq.Emit(londo.RenewEvent{}, e); err != nil {
			londo.CheckFatalError(err)
		}
	}
}

func main() {
	londo.ConfigureLogging(log.DebugLevel)

	log.Info("Starting Database Daemon...")

	log.Info("Reading configuration...")
	c, err := londo.ReadConfig()
	londo.CheckFatalError(err)

	db, err := londo.NewDBConnection(c)
	londo.CheckFatalError(err)
	log.Infof("Connecting to %v database", db.Name)

	log.Info("Connecting to RabbitMQ...")

	lch := londo.CreateLogChannel()

	mq, err := londo.NewMQConnection(c, db, lch)
	londo.CheckFatalError(err)

	log.Info("Declaring Exchange...")
	err = mq.ExchangeDeclare()
	londo.CheckFatalError(err)

	cron := gron.New()
	cron.AddFunc(gron.Every(1*time.Minute), func() {
		PublishExpiringCerts(db, mq)
	})

	cron.Start()

	_, err = mq.QueueDeclare(londo.DeleteSubjEvent)
	londo.CheckFatalError(err)

	err = mq.QueueBind(londo.DeleteSubjEvent, londo.DeleteSubjEvent)
	londo.CheckFatalError(err)

	go mq.Consume(londo.DeleteSubjEvent)

	for {
		select {
		case i := <-lch.Info:
			log.Info(i)
		case w := <-lch.Warn:
			log.Warn(w)
		case e := <-lch.Err:
			log.Error(e)
		case a := <-lch.Abort:
			log.Error(a)
			break
		}
	}

	log.Info("Disconnecting from the database")
	db.Disconnect()

	log.Info("Shutting down RabbitMQ connection")
	mq.Shutdown()
}
