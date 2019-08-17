package main

import (
	"github.com/alexyermolaev/londo"
	log "github.com/sirupsen/logrus"
)

// lchecker checks expring certificates

func main() {
	londo.ConfigureLogging(log.DebugLevel)

	log.Info("Starting Database Daemon...")

	log.Info("Reading configuration")
	c, err := londo.ReadConfig()
	londo.CheckFatalError(err)

	db, err := londo.NewDBConnection(c)
	londo.CheckFatalError(err)
	log.Infof("Connecting to %v database", db.Name)

	mq, err := londo.NewProducer(c)
	londo.CheckFatalError(err)

	exp, err := db.FindExpiringSubjects(720)
	londo.CheckFatalError(err)

	for _, e := range exp {
		log.Infof("%v, %v", e.Subject, e.NotAfter)
		if err := mq.EmitRenew(e); err != nil {
			londo.CheckFatalError(err)
		}
	}

	log.Info("Disconnecting from the database")
	db.Disconnect()

	log.Info("Shutting down RabbitMQ connection")
	mq.Shutdown()
}
