package main

import (
	"github.com/alexyermolaev/londo"
	"github.com/streadway/amqp"
)

func main() {

	londo.S("renew").
		NewAMQPConnection().
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect).
		Declare(
			londo.RenewExchange,
			londo.RenewQueue,
			amqp.ExchangeDirect).
		ConsumeRenew(londo.RenewQueue).
		Run()

}
