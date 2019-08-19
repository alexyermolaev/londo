package main

import "github.com/alexyermolaev/londo"

func main() {

	londo.Start("renew", false, londo.RenewService)
	//log.Info("Shutting down RabbitMQ connection..")
	//mq.Shutdown()
}
