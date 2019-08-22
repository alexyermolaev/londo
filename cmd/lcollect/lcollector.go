package main

import (
	"github.com/alexyermolaev/londo"
	"github.com/streadway/amqp"
)

func main() {
	londo.S("collector").
		AMQPConnection().
		RestAPIClient().
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect, nil).
		Declare(
			londo.CollectExchange,
			londo.CollectQueue,
			amqp.ExchangeDirect, nil).
		ConsumeCollect().
		Run()

}
