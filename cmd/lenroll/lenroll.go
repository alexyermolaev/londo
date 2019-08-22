package main

import (
	"github.com/alexyermolaev/londo"
	"github.com/streadway/amqp"
)

func main() {
	londo.S("enroll").
		AMQPConnection().
		RestAPIClient().
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect, nil).
		Declare(
			londo.EnrollExchange,
			londo.EnrollQueue,
			amqp.ExchangeDirect, nil).
		Declare(
			londo.CollectExchange,
			londo.CollectQueue,
			amqp.ExchangeDirect, nil).
		ConsumeEnroll().
		Run()
}
