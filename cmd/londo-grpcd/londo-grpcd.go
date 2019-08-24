package main

import (
	"github.com/alexyermolaev/londo"
	"github.com/streadway/amqp"
)

func main() {

	londo.S("gRPC server").
		AMQPConnection().
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect, nil).
		DeclareExchange(
			londo.GRPCServerExchange,
			amqp.ExchangeDirect).
		GRPCServer().
		Run()
}
