package londo

import (
	"github.com/streadway/amqp"
	"strconv"
)

const (
	RenewEventName     = "renew"
	RevokeEventName    = "revoke"
	EnrollEventName    = "enroll"
	DeleteSubjEvent    = "delete"
	CompleteEnrollName = "complete"
	CSREventName       = "newcsr"
	CollectEventName   = "collect"
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

func (a *AMQP) Emit(msg amqp.Publishing, exchange string, key string) error {
	ch, err := a.connection.Channel()
	defer ch.Close()
	if err != nil {
		return err
	}

	return ch.Publish(exchange, key, false, false, msg)
}

func (a *AMQP) Consume(event Event, queue string) error {
	ch, err := a.connection.Channel()
	defer ch.Close()
	if err != nil {
		return err
	}

	delivery, err := ch.Consume(
		queue, "", false, true, false, false, nil)
	if err != nil {
		return err
	}

	for d := range delivery {
	}
}
