package londo

import (
	"encoding/json"
	"errors"
	"github.com/streadway/amqp"
	"net/http"
	"strconv"
	"time"

	"github.com/roylee0704/gron"
	log "github.com/sirupsen/logrus"
)

const (
	RenewExchange = "renew-rpc"
	RenewQueue    = "renew"

	DbReplyExchange = "db-rpc"
	DbReplyQueue    = "db-rpc-replies"

	DeleteCommand = "delete_subj"
)

type Londo struct {
	Name       string
	Db         *MongoDB
	AMQP       *AMQP
	Config     *Config
	LogChannel *LogChannel
}

func (l *Londo) PublishExpiringCerts(exchange string, queue string, reply string) *Londo {
	cron := gron.New()

	cron.AddFunc(gron.Every(1*time.Minute), func() {
		exp, err := l.Db.FindExpiringSubjects(720)
		CheckFatalError(err)

		for _, e := range exp {

			re := RenewEvent{
				Subject:  e.Subject,
				CertID:   e.CertID,
				AltNames: e.AltNames,
				Targets:  e.Targets,
			}

			j, err := json.Marshal(&re)
			if err != nil {
				l.LogChannel.Err <- err
			}

			if err = l.AMQP.Emit(
				exchange,
				queue,
				amqp.Publishing{
					Headers:     nil,
					ContentType: "application/json",
					ReplyTo:     reply,
					Expiration:  strconv.Itoa(int(time.Now().Add(1 * time.Minute).Unix())),
					Timestamp:   time.Time{},
					Body:        j,
				}); err != nil {
				l.LogChannel.Err <- err
			} else {
				l.LogChannel.Info <- "published " + e.Subject
			}
		}
	})

	cron.Start()

	return l
}

func (l *Londo) ConsumeRenew(queue string) *Londo {
	go l.AMQP.Consume(queue, func(d amqp.Delivery) error {

		rest := NewRestClient(l.Config)

		var s Subject
		err := json.Unmarshal(d.Body, &s)
		if err != nil {
			err = d.Reject(false)
			return err
		}

		res, err := rest.Revoke(s.CertID)

		// TODO: Response result processing needs to be elsewhere
		if err != nil {
			err = d.Reject(true)
			return err
		}

		if res.StatusCode() != http.StatusNoContent {
			err = d.Reject(true)
			return errors.New("remote returned " + strconv.Itoa(res.StatusCode()) + " status code")
		}

		// TODO: Generate an event to re-register subject

		if d.ReplyTo != "" {
			// TODO: Needs to be extract into its own method
			e := DeleteSubjEvenet{
				CertID: s.CertID,
			}

			// The error should never happen, or should it?
			j, err := json.Marshal(&e)
			if err != nil {
				return err
			}

			if err := l.AMQP.Emit(
				"",
				d.ReplyTo,
				amqp.Publishing{
					ContentType: "application/json",
					Type:        DeleteCommand,
					Body:        j,
				}); err != nil {
				return err
			} else {
				l.LogChannel.Info <- "requesting deletion of " + s.Subject
			}
		}

		l.LogChannel.Info <- "subject " + s.Subject + " received"
		err = d.Ack(false)
		return err
	})

	return l
}

func (l *Londo) ConsumeDbRPC(queue string) *Londo {
	go l.AMQP.Consume(queue, func(d amqp.Delivery) error {

		switch d.Type {
		case DeleteCommand:

			var e DeleteSubjEvenet
			if err := json.Unmarshal(d.Body, &e); err != nil {
				d.Reject(false)
				return err
			}

			if err := l.Db.DeleteSubject(e.CertID); err != nil {
				d.Reject(false)
				return err
			}

			l.LogChannel.Info <- "certificate " + strconv.Itoa(e.CertID) + " has been deleted."

		default:
			l.LogChannel.Warn <- "unknown command received"
		}

		d.Ack(false)
		return nil
	})

	return l
}

func (l *Londo) NewAMQPConnection() *Londo {
	var err error

	log.Info("Connecting to RabbitMQ...")
	l.AMQP, err = NewMQConnection(l.Config, l.Db, l.LogChannel)
	CheckFatalError(err)

	return l
}

func (l *Londo) Declare(exchange string, queue string, kind string) *Londo {
	ch, err := l.AMQP.connection.Channel()
	defer ch.Close()
	CheckFatalError(err)

	err = ch.ExchangeDeclare(
		exchange, kind, true, false, false, false, nil)
	CheckFatalError(err)

	log.Infof("Declaring %s queue...", queue)
	_, err = ch.QueueDeclare(
		queue, false, false, false, false, nil)
	CheckFatalError(err)

	log.Infof("Binding to %s queue...", queue)
	err = ch.QueueBind(queue, queue, exchange, false, nil)
	CheckFatalError(err)

	return l
}

func (l *Londo) DbService() *Londo {
	var err error

	log.Info("Connecting to the database...")
	l.Db, err = NewDBConnection(l.Config)
	CheckFatalError(err)

	return l
}

func S(name string) *Londo {
	l := &Londo{
		Name: name,
	}

	ConfigureLogging(log.DebugLevel)

	log.Infof("Starting %s service...", l.Name)

	log.Info("Reading configuration...")

	var err error

	l.Config, err = ReadConfig()
	CheckFatalError(err)

	l.LogChannel = CreateLogChannel()

	return l
}

func (l *Londo) Run() {
	for {
		select {
		case i := <-l.LogChannel.Info:
			log.Info(i)
		case w := <-l.LogChannel.Warn:
			log.Warn(w)
		case e := <-l.LogChannel.Err:
			log.Error(e)
		case a := <-l.LogChannel.Abort:
			log.Error(a)
			break
		}
	}
}
