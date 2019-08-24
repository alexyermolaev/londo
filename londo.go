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
	"google.golang.org/grpc/reflection"
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

	// Commands
	// Db
	DbDeleteSubjComd        = "subj.delete"
	DbAddSubjComd           = "subj.add"
	DbUpdateSubjComd        = "subj.update"
	DbGetSubjectComd        = "subj.get"
	DbGetSubjectByTargetCmd = "subj.get.taarget"

	// Tell consumer to close channel
	CloseChannelCmd = "stop"

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

	log.Infof("declaring %s exchange...", exchange)
	err = ch.ExchangeDeclare(
		exchange, kind, true, false, false, false, nil)
	fail(err)

	log.Infof("declaring %s queue...", queue)
	q, err := ch.QueueDeclare(
		queue, false, false, false, false, args)
	fail(err)

	log.Infof("binding to %s queue...", q.Name)
	err = ch.QueueBind(queue, queue, exchange, false, nil)
	fail(err)

	return l
}

// TODO: refactor
func (l *Londo) DeclareExchange(exchange string, kind string) *Londo {
	ch, err := l.AMQP.connection.Channel()
	defer ch.Close()
	fail(err)

	log.Infof("declaring %s exchange...", exchange)
	err = ch.ExchangeDeclare(
		exchange, kind, true, false, false, false, nil)
	fail(err)

	return l
}

// TODO: refactor
func (l *Londo) DeclareBindQueue(exchange string, queue string) error {
	ch, err := l.AMQP.connection.Channel()
	defer ch.Close()
	fail(err)

	log.Infof("declaring %s queue...", queue)
	q, err := ch.QueueDeclare(queue, false, true, false, false, nil)
	if err != nil {
		return err
	}

	log.Infof("binding to %s queue...", q.Name)
	err = ch.QueueBind(queue, queue, exchange, false, nil)

	return err
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
	londopb.RegisterCertServiceServer(srv, &GRPCServer{
		Londo: l,
	})

	reflection.Register(srv)

	go func() {
		log.Infof("server is running on port %d", l.Config.GRPC.Port)
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
