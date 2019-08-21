package main

import (
	"github.com/alexyermolaev/londo"
	"github.com/streadway/amqp"
)

func main() {
	londo.S("enroll").
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect, nil).
		Declare(
			londo.EnrollExchange,
			londo.EnrollQueue,
			amqp.ExchangeDirect, nil).
		Run()
}
