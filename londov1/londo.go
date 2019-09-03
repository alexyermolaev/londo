package londov1

import (
	"github.com/alexyermolaev/londo"
	"github.com/alexyermolaev/londo/londov1/logger"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/streadway/amqp"
	"os"
	"os/signal"
	"strconv"
)

var (
	log   *logrus.Entry
	mqlog *logrus.Entry

	mqconn *amqp.Connection
	config londo.Config
)

type Londo struct {
	Name string
}

func Initialize(name string) *Londo {

	l := new(Londo)
	l.Name = name
	log = logger.SetupLogging(l.Name, false)
	log.Info(logger.Initializing)

	v := viper.New()
	v.AddConfigPath("config")
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	log.Info(logger.ReadingConf)
	if err := v.ReadInConfig(); err != nil {
		log.Fatal(err)
	}

	log.WithFields(logrus.Fields{logger.File: v.ConfigFileUsed()}).Info(logger.Unmarshalling)
	if err := v.Unmarshal(&config); err != nil {
		log.Fatal(err)
	}

	return l
}

func (l *Londo) AMQPService() *Londo {
	mqlog = log.WithFields(logrus.Fields{logger.Connection: "rabbitmq"})

	mqlog.Info(logger.Connecting)
	conn, err := amqp.Dial("amqp://" + config.AMQP.Username + ":" + config.AMQP.Password +
		"@" + config.Hostname + ":" + strconv.Itoa(config.AMQP.Port))
	if err != nil {
		mqlog.Fatal(err)
	}

	mqconn = conn
	mqlog.Info(logger.Connected)

	mqlog = mqlog.WithFields(logrus.Fields{logger.Exchange: config.AMQP.Exchange})
	mqlog.Info(logger.Declaring)

	ch, err := mqconn.Channel()
	if err != nil {
		mqlog.Fatal(err)
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(
		config.AMQP.Exchange, "topic", true, false, false, false, nil); err != nil {
		mqlog.Fatal(err)
	}
	mqlog.Info(logger.Success)

	return l
}

func (l *Londo) DeclareAndBindQueue(queue string) *Londo {
	mqlog = mqlog.WithFields(logrus.Fields{logger.Queue: queue})

	ch, err := mqconn.Channel()
	if err != nil {
		mqlog.Fatal(err)
	}
	defer ch.Close()

	mqlog.Info(logger.Declaring)
	q, err := ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		mqlog.Fatal(err)
	}

	mqlog.Info(logger.Binding)
	if err := ch.QueueBind(q.Name, q.Name, config.AMQP.Exchange, false, nil); err != nil {
		mqlog.Fatal(err)
	}

	return l
}

func (l *Londo) Run() {
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)

	<-s

	log.Info(logger.ShuttingDown)
	l.shutdown(0)
}

func (l *Londo) shutdown(code int) {
	if mqconn != nil {
		mqconn.Close()
	}
}
