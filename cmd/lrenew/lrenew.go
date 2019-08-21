package main

import (
	"github.com/alexyermolaev/londo"
	"github.com/streadway/amqp"
	"time"
)

func main() {

	londo.S("renew").
		NewAMQPConnection().
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
		Declare(
			londo.EnrollExchange,
			londo.EnrollQueue,
			amqp.ExchangeDirect, nil).
		ConsumeRenew(londo.RenewQueue).
		Run()

}
