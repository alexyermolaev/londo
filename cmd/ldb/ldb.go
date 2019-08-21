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

	londo.S("db").
		DbService().
		NewAMQPConnection().
		Declare(reqExchange, reqQueue, amqp.ExchangeDirect).
		Declare(replExchange, replQueue, amqp.ExchangeDirect).
		PublishExpiringCerts(reqExchange, reqQueue, replQueue).
		Run()
}
