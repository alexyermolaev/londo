package londo

import (
	"time"

	"github.com/roylee0704/gron"
	log "github.com/sirupsen/logrus"
)

type Londo struct {
	Name       string
	db         *MongoDB
	amqp       *AMQP
	config     *Config
	logChannel *LogChannel
}

func (l *Londo) PublishExpiringCerts() *Londo {
	cron := gron.New()

	cron.AddFunc(gron.Every(1*time.Minute), func() {
		exp, err := l.db.FindExpiringSubjects(720)
		CheckFatalError(err)

		for _, e := range exp {
			log.Infof("%v, %v", e.Subject, e.NotAfter)
			if err := l.amqp.Emit(RenewEvent{}, e); err != nil {
				CheckFatalError(err)
			}
		}
	})

	cron.Start()

	return l
}

func (l *Londo) RenewService() *Londo {
	log.Infof("Declaring %s queue...", RenewEventName)
	_, err := l.amqp.QueueDeclare(RenewEventName)
	CheckFatalError(err)

	log.Infof("Binding exchange to %s queue...", RenewEventName)
	CheckFatalError(err)

	go l.amqp.Consume(RenewEventName)

	return l
}

func (l *Londo) DeleteSubjService() *Londo {
	_, err := l.amqp.QueueDeclare(DeleteSubjEvent)
	CheckFatalError(err)

	err = l.amqp.QueueBind(DeleteSubjEvent, DeleteSubjEvent)
	CheckFatalError(err)

	go l.amqp.Consume(DeleteSubjEvent)

	return l
}

func (l *Londo) RabbitMQService() *Londo {
	var err error

	log.Info("Connecting to RabbitMQ...")
	l.amqp, err = NewMQConnection(l.config, l.db, l.logChannel)
	CheckFatalError(err)

	log.Infof("Declaring %s exchange...", l.amqp.exchange)
	err = l.amqp.ExchangeDeclare()
	CheckFatalError(err)

	return l
}

func (l *Londo) DbService() *Londo {
	var err error

	log.Info("Connecting to the database...")
	l.db, err = NewDBConnection(l.config)
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

	l.config, err = ReadConfig()
	CheckFatalError(err)

	l.logChannel = CreateLogChannel()

	return l
}

func (l *Londo) Run() {
	for {
		select {
		case i := <-l.logChannel.Info:
			log.Info(i)
		case w := <-l.logChannel.Warn:
			log.Warn(w)
		case e := <-l.logChannel.Err:
			log.Error(e)
		case a := <-l.logChannel.Abort:
			log.Error(a)
			break
		}
	}
}
