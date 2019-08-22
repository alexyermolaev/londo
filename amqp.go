package londo

import (
	"strconv"

	"github.com/streadway/amqp"
)

type AMQP struct {
	connection *amqp.Connection
	logChannel *LogChannel
	config     *Config
	db         *MongoDB
}

func (a *AMQP) Shutdown() {
	a.connection.Close()
}

func NewMQConnection(c *Config, db *MongoDB, lch *LogChannel) (*AMQP, error) {
	mq := &AMQP{
		config:     c,
		logChannel: lch,
		db:         db,
	}

	var err error

	mq.connection, err = amqp.Dial(
		"AMQP://" + c.AMQP.Username + ":" + c.AMQP.Password + "@" + c.AMQP.Hostname + ":" +
			strconv.Itoa(c.AMQP.Port))
	if err != nil {
		return mq, err
	}

	return mq, err
}

func (a *AMQP) Emit(exchange string, key string, msg amqp.Publishing) error {
	ch, err := a.connection.Channel()
	defer ch.Close()
	if err != nil {
		return err
	}

	return ch.Publish(exchange, key, false, false, msg)
}

func (a *AMQP) Consume(queue string, f func(d amqp.Delivery) error) {
	a.logChannel.Info <- "consuming " + queue + " queue."
	ch, err := a.connection.Channel()
	defer ch.Close()
	if err != nil {
		a.logChannel.Err <- err
		return
	}

	delivery, err := ch.Consume(
		queue, "", false, true, false, false, nil)
	if err != nil {
		a.logChannel.Err <- err
	}

	for d := range delivery {
		err := f(d)
		if err != nil {
			a.logChannel.Err <- err
		} else {
			d.Ack(false)
		}
	}
}
