package main

import "github.com/alexyermolaev/londo"

func main() {

	londo.Start("db", true, londo.DbService)

	//log.Info("Disconnecting from the database")
	//db.Disconnect()
	//
	//log.Info("Shutting down RabbitMQ connection")
	//mq.Shutdown()
}
