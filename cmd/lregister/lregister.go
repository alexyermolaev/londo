package main

import (
	"github.com/alexyermolaev/londo"
	log "github.com/sirupsen/logrus"
)

func main() {
	londo.ConfigureLogging(log.DebugLevel)

	log.Info("Starting L-Register daemon...")

	log.Info("Reading configuration")
	c, err := londo.ReadConfig()
	londo.CheckFatalError(err)

	log.Info("Connecting to RabbitMQ...")
	mq, err := londo.NewMQConnection(c)
	londo.CheckFatalError(err)

	_, err = mq.QueueDeclare(londo.RenewEventName)
	londo.CheckFatalError(err)

	err = mq.QueueBind(londo.RenewEventName, londo.RenewEventName)
	londo.CheckFatalError(err)

	_, err = mq.QueueDeclare(londo.RevokeEventName)
	londo.CheckFatalError(err)

	err = mq.QueueBind(londo.RevokeEventName, londo.RevokeEventName)
	londo.CheckFatalError(err)

	go mq.Consume(londo.RenewEventName)

	mq.Consume(londo.RevokeEventName)

	log.Info("Shutting down RabbitMQ connection..")
	mq.Shutdown()
}
