package londo

import (
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/alexyermolaev/londo/londopb"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"google.golang.org/grpc"
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

	GRPCServerExchange = "grpc"

	// Db Commands
	DbDeleteSubjCommand = "subj.delete"
	DbAddSubjCommand    = "subj.add"
	DbUpdateSubjCommand = "subj.update"
	DbGetSubjectCommand = "subj.get"

	ContentType = "application/json"
)

type Londo struct {
	Name       string
	Db         *MongoDB
	AMQP       *AMQP
	GRPC       *GRPCServer
	Config     *Config
	RestClient *RestAPI
}

func (l *Londo) AMQPConnection() *Londo {
	var err error

	log.Info("Connecting to RabbitMQ...")
	l.AMQP, err = NewMQConnection(l.Config, l.Db)
	fail(err)

	return l
}

func (l *Londo) Declare(exchange string, queue string, kind string, args amqp.Table) *Londo {
	ch, err := l.AMQP.connection.Channel()
	defer ch.Close()
	fail(err)

	err = ch.ExchangeDeclare(
		exchange, kind, true, false, false, false, nil)
	fail(err)

	log.Infof("Declaring %s queue...", queue)
	_, err = ch.QueueDeclare(
		queue, false, false, false, false, args)
	fail(err)

	log.Infof("Binding to %s queue...", queue)
	err = ch.QueueBind(queue, queue, exchange, false, nil)
	fail(err)

	return l
}

func (l *Londo) DbService() *Londo {
	var err error

	log.Info("Connecting to the database...")
	l.Db, err = NewDBConnection(l.Config)
	fail(err)

	return l
}

func S(name string) *Londo {
	ConfigureLogging(log.DebugLevel)

	l := &Londo{
		Name: name,
	}

	var err error

	log.Info("Reading configuration...")
	l.Config, err = ReadConfig()
	fail(err)

	log.Info("Starting " + l.Name + " service...")

	return l
}

func (l *Londo) GRPCServer() *Londo {
	log.Info("initializing grpc...")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", l.Config.GRPC.Port))
	fail(err)

	opts := []grpc.ServerOption{}
	srv := grpc.NewServer(opts...)
	londopb.RegisterCertServiceServer(srv, &GRPCServer{})

	go func() {
		if err := srv.Serve(lis); err != nil {
			fail(err)
		}
	}()

	return l
}

func (l *Londo) Run() {
	//if l.Config.Debug == 1 {
	//	ConfigureLogging(log.DebugLevel)
	//	log.Info("enabled debug level")
	//}

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)

	<-s
	log.Info("Goodbye, Captain Sheridan!")
	l.shutdown(0)
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

func (l *Londo) DbGetSubjectCommand() string {
	return DbGetSubjectCommand
}
