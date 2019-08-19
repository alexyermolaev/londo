package londo

import (
	"time"

	"github.com/roylee0704/gron"
	log "github.com/sirupsen/logrus"
)

const (
	DbService    = 1
	RenewService = 2
)

type Londo struct {
	Name   string
	db     *MongoDB
	amqp   *AMQP
	config *Config
}

func (l *Londo) publishExpiringCerts() {
	exp, err := l.db.FindExpiringSubjects(720)
	CheckFatalError(err)

	for _, e := range exp {
		log.Infof("%v, %v", e.Subject, e.NotAfter)
		if err := l.amqp.Emit(RenewEvent{}, e); err != nil {
			CheckFatalError(err)
		}
	}
}

func (l *Londo) dbService() error {
	cron := gron.New()
	cron.AddFunc(gron.Every(1*time.Minute), func() {
		l.publishExpiringCerts()
	})
	cron.Start()

	_, err := l.amqp.QueueDeclare(DeleteSubjEvent)
	if err != nil {
		return err
	}

	err = l.amqp.QueueBind(DeleteSubjEvent, DeleteSubjEvent)
	if err != nil {
		return err
	}

	go l.amqp.Consume(DeleteSubjEvent)

	return nil
}

func (l *Londo) renewService() error {
	log.Infof("Declaring %s queue...", RenewEventName)
	_, err := l.amqp.QueueDeclare(RenewEventName)
	if err != nil {
		return err
	}

	log.Infof("Binding exchange to %s queue...", RenewEventName)
	if err = l.amqp.QueueBind(RenewEventName, RenewEventName); err != nil {
		return err
	}

	return nil
}

func Start(name string, db bool, t int) {
	l := &Londo{
		Name: name,
	}

	ConfigureLogging(log.DebugLevel)

	log.Infof("Starting %s service...", l.Name)

	log.Info("Reading configuration...")

	var err error

	l.config, err = ReadConfig()
	CheckFatalError(err)

	if db {
		l.db, err = NewDBConnection(l.config)
		CheckFatalError(err)
		log.Infof("Connecting to %s database", l.db.Name)
	}

	lch := CreateLogChannel()

	log.Info("Connecting to RabbitMQ...")
	l.amqp, err = NewMQConnection(l.config, l.db, lch)
	CheckFatalError(err)

	log.Infof("Declaring %s exchange...", l.amqp.exchange)
	err = l.amqp.ExchangeDeclare()
	CheckFatalError(err)

	switch t {
	case DbService:
		err = l.dbService()
		CheckFatalError(err)
	case RenewService:
		err = l.renewService()
		CheckFatalError(err)
	}

	for {
		select {
		case i := <-lch.Info:
			log.Info(i)
		case w := <-lch.Warn:
			log.Warn(w)
		case e := <-lch.Err:
			log.Error(e)
		case a := <-lch.Abort:
			log.Error(a)
			break
		}
	}
}
