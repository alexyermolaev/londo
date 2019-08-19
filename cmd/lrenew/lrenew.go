package main

import "github.com/alexyermolaev/londo"

func main() {

	londo.S("renew").
		RabbitMQService().
		RenewService().
		Run()
	//log.Info("Shutting down RabbitMQ connection..")
	//mq.Shutdown()
}
