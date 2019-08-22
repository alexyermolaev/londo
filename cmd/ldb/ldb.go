package main

import (
	"github.com/alexyermolaev/londo"
	"github.com/streadway/amqp"
	"time"
)

func main() {

	londo.S("db").
		DbService().
		AMQPConnection().
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect, nil).
		Declare(
			londo.RenewExchange,
			londo.RenewQueue,
			amqp.ExchangeDirect, amqp.Table{
				"x-message-ttl": int(59 * time.Second / time.Millisecond),
			}).
		ConsumeDbRPC().
		PublishExpiringCerts(
			londo.RenewExchange,
			londo.RenewQueue,
			londo.DbReplyQueue).
		Run()
}
