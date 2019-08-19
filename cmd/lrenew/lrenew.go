package main

import "github.com/alexyermolaev/londo"

func main() {

	londo.S("renew").
		RenewService().
		Run()
	//log.Info("Shutting down RabbitMQ connection..")
	//mq.Shutdown()
}
