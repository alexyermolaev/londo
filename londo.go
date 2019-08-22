package londo

import (
	"os"
	"os/signal"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

const (
	EnrollExchange = "enroll-rpc"
	EnrollQueue    = "enroll"

	RenewExchange = "renew-rpc"
	RenewQueue    = "renew"

	CollectExchange = "collect-rpc"
	CollectQueue    = "collect"

	DbReplyExchange = "db-rpc"
	DbReplyQueue    = "db-rpc-replies"

	// Db Commands
	DbDeleteSubjCommand = "delete_subj"
	DbAddSubjCommand    = "add_subj"
	DbUpdateSubjCommand = "update_subj"

	ContentType = "application/json"
)

type Londo struct {
	Name       string
	Db         *MongoDB
	AMQP       *AMQP
	Config     *Config
	Log        *Log
	RestClient *RestAPI
}

func (l *Londo) AMQPConnection() *Londo {
	var err error

	log.Info("Connecting to RabbitMQ...")
	l.AMQP, err = NewMQConnection(l.Config, l.Db, l.Log)
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
	ConfigureLogging(log.DebugLevel)

	l := &Londo{
		Name: name,
		Log:  CreateLogChannel(),
	}

	var err error

	log.Info("Reading configuration...")
	l.Config, err = ReadConfig()
	CheckFatalError(err)

	log.Info("Starting " + l.Name + " service...")

	return l
}

func (l *Londo) Run() {
	//if l.Config.Debug == 1 {
	//	ConfigureLogging(log.DebugLevel)
	//	log.Info("enabled debug level")
	//}

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)

	for {
		select {
		case _ = <-s:
			log.Info("Goodbye, Captain Sheridan!")
			l.shutdown(0)
		case m := <-l.Log.Info:
			log.Info(m)
		case m := <-l.Log.Warn:
			log.Warn(m)
		case m := <-l.Log.Debug:
			log.Debug(m)
		case m := <-l.Log.Err:
			log.Error(m)
		case m := <-l.Log.Abort:
			log.Error(m)
			l.shutdown(1)
		}
	}
}

func (l *Londo) RestAPIClient() *Londo {
	l.RestClient = NewRestClient(l.Config)
	return l
}

func (l *Londo) shutdown(code int) {
	if l.Db == nil {
		os.Exit(code)
	}
	if err := l.Db.Disconnect(); err != nil {
		log.Error(err)
		code = 1
	}
	os.Exit(code)
}
