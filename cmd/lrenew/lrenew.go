package main

import "github.com/alexyermolaev/londo"

func main() {

	londo.S("renew").
		RabbitMQService().
		EventService(londo.RenewEventName).
		Run()
	//log.Info("Shutting down RabbitMQ connection..")
	//mq.Shutdown()
}
