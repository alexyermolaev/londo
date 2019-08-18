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

	_, err = mq.QueueDeclare("renew")
	londo.CheckFatalError(err)

	err = mq.QueueBind("renew", "renew")
	londo.CheckFatalError(err)

	err = mq.Consume("renew")
	londo.CheckFatalError(err)

	log.Info("Shutting down RabbitMQ connection..")
	mq.Shutdown()
}
