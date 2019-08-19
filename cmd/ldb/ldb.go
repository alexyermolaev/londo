package main

import "github.com/alexyermolaev/londo"

func main() {

	londo.S("db").
		DbService().
		RabbitMQService().
		EventService(londo.DeleteSubjEvent).
		PublishExpiringCerts().
		Run()
	//log.Info("Disconnecting from the database")
	//db.Disconnect()
	//
	//log.Info("Shutting down RabbitMQ connection")
	//mq.Shutdown()
}
