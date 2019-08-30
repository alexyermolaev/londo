package londo

import (
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

type AMQP struct {
	connection *amqp.Connection
	config     *Config
	db         *MongoDB
}

func (a *AMQP) Shutdown() {
	a.connection.Close()
}

func NewMQConnection(c *Config, db *MongoDB) (*AMQP, error) {
	mq := &AMQP{
		config: c,
		db:     db,
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

func (a *AMQP) Consume(queue string, wg *sync.WaitGroup, f func(d amqp.Delivery) (error, bool)) {
	if wg != nil {
		defer wg.Done()
	}

	log.WithFields(logrus.Fields{logQueue: queue}).Info("consuming")

	ch, err := a.connection.Channel()
	defer ch.Close()
	if err != nil {
		log.Error(err)
		return
	}

	delivery, err := ch.Consume(
		queue, "", false, true, false, false, nil)
	if err != nil {
		log.Error(err)
	}

	for d := range delivery {
		err, abort := f(d)
		if err != nil {
			log.Error(err)
		} else {
			d.Ack(false)
		}

		if abort {
			break
		}
	}

	log.WithFields(logrus.Fields{logQueue: queue}).Info("closed")
}
