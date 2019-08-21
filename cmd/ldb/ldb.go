package main

import (
	"github.com/alexyermolaev/londo"
	"github.com/streadway/amqp"
)

func main() {

	londo.S("db").
		DbService().
		NewAMQPConnection().
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect).
		Declare(
			londo.RenewExchange,
			londo.RenewQueue,
			amqp.ExchangeDirect).
		PublishExpiringCerts(
			londo.RenewExchange,
			londo.RenewQueue,
			londo.DbReplyQueue).
		Run()
}
