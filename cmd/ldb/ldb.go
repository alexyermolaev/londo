package main

import (
	"time"

	"github.com/alexyermolaev/londo"
	"github.com/roylee0704/gron"
	log "github.com/sirupsen/logrus"
)

func PublishExpiringCerts(db *londo.MongoDB, mq *londo.AMQP) {
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
	mq, err := londo.NewMQConnection(c)
	londo.CheckFatalError(err)

	log.Info("Declaring Exchange...")
	err = mq.ExchangeDeclare()
	londo.CheckFatalError(err)

	cron := gron.New()
	cron.AddFunc(gron.Every(1*time.Hour), func() {
		PublishExpiringCerts(db, mq)
	})

	cron.Start()

	select {}

	log.Info("Disconnecting from the database")
	db.Disconnect()

	log.Info("Shutting down RabbitMQ connection")
	mq.Shutdown()
}
