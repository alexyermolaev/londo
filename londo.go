package londo

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/roylee0704/gron"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

const (
	EnrollExchange = "enroll-rpc"
	EnrollQueue    = "enroll"

	RenewExchange = "renew-rpc"
	RenewQueue    = "renew"

	DbReplyExchange = "db-rpc"
	DbReplyQueue    = "db-rpc-replies"

	DeleteCommand = "delete_subj"

	ContentType = "application/json"
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
					ContentType:   ContentType,
					ReplyTo:       reply,
					CorrelationId: e.ID.Hex(),
					Expiration:    strconv.Itoa(int(time.Now().Add(1 * time.Minute).Unix())),
					Timestamp:     time.Time{},
					Body:          j,
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

func (l *Londo) PublishNewSubject(exchange string, queue string, s *Subject) *Londo {

	e := EnrollEvent{
		Subject:  s.Subject,
		AltNames: s.AltNames,
		Targets:  s.Targets,
	}

	j, err := json.Marshal(&e)

	if err := l.AMQP.Emit(
		exchange,
		queue,
		amqp.Publishing{
			ContentType: ContentType,
			Body:        j,
		}); err != nil {
		l.LogChannel.Err <- err
	} else {
		l.LogChannel.Info <- "enrolling new subject: " + e.Subject
	}
}

// TODO: Finish consumer
func (l *Londo) ConsumeEnroll(queue string) *Londo {
	go l.AMQP.Consume(queue, func(d amqp.Delivery) error {
		return errors.New("not implemented")
	})

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
					ContentType:   "application/json",
					Type:          DeleteCommand,
					CorrelationId: d.CorrelationId,
					Body:          j,
				}); err != nil {
				return err
			} else {
				l.LogChannel.Info <- "requesting deletion of " + s.Subject
			}
		}

		// TODO: Publish enroll new cert

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

			if err := l.Db.DeleteSubject(d.CorrelationId, e.CertID); err != nil {
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

func (l *Londo) Declare(exchange string, queue string, kind string, args amqp.Table) *Londo {
	ch, err := l.AMQP.connection.Channel()
	defer ch.Close()
	CheckFatalError(err)

	err = ch.ExchangeDeclare(
		exchange, kind, true, false, false, false, nil)
	CheckFatalError(err)

	log.Infof("Declaring %s queue...", queue)
	_, err = ch.QueueDeclare(
		queue, false, false, false, false, args)
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
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)

	for {
		select {
		case _ = <-s:
			log.Info("Goodbye, Captain Sheridan!")
			l.shutdown(0)
		case m := <-l.LogChannel.Info:
			log.Info(m)
		case m := <-l.LogChannel.Warn:
			log.Warn(m)
		case m := <-l.LogChannel.Err:
			log.Error(m)
		case m := <-l.LogChannel.Abort:
			log.Error(m)
			l.shutdown(1)
		}
	}

}

func (l *Londo) shutdown(code int) {
	if err := l.Db.Disconnect(); err != nil {
		log.Error(err)
		code = 1
	}
	os.Exit(code)
}
