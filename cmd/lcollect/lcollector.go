package main

import (
	"github.com/alexyermolaev/londo"
	"github.com/streadway/amqp"
)

// TODO: need to code scheduler for period collection of certificates

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
