package main

import (
	"github.com/alexyermolaev/londo"
	"github.com/streadway/amqp"
)

func main() {

	reqExchange := "rpc-requests"
	reqQueue := "rpc-events"

	replExchange := "rpc-replies"
	replQueue := "rpc-reply"

	londo.S("renew").
		NewAMQPConnection().
		Declare(reqExchange, reqQueue, amqp.ExchangeDirect).
		Declare(replExchange, replQueue, amqp.ExchangeDirect).
		ConsumeRenew(reqQueue).
		Run()

}
