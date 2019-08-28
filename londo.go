package londo

import (
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/alexyermolaev/londo/londopb"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
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

	CheckExchange = "check-rpc"
	CheckQueue    = "check"

	DbReplyExchange = "db-rpc"
	DbReplyQueue    = "db-rpc-replies"

	GRPCServerExchange = "grpc"

	// Commands
	// Db
	DbDeleteSubjComd         = "subj.delete"
	DbAddSubjComd            = "subj.add"
	DbUpdateSubjComd         = "subj.update"
	DbGetSubjectComd         = "subj.get"
	DbGetAllSubjectsCmd      = "subj.get.all"
	DbGetSubjectByTargetCmd  = "subj.get.taarget"
	DbGetExpiringSubjectsCmd = "subj.get.expiring"

	// Tell consumer to close channel
	CloseChannelCmd = "stop"

	ContentType = "application/json"
)

var (
	Debug     bool
	ScanHours int

	cfg *Config
	err error
)

func init() {

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	log.Info("reading configuration")
	cfg, err = ReadConfig()
	if err != nil {
		log.Fatal("cannot read config")
		os.Exit(1)
	}
	if cfg.Debug == 1 {
		log.Warn("debugging is on")
		log.SetLevel(log.DebugLevel)
	}
}

type Londo struct {
	Name       string
	Db         *MongoDB
	AMQP       *AMQP
	GRPC       *GRPCServer
	RestClient *RestAPI
}

func (l *Londo) AMQPConnection() *Londo {

	log.Info("Connecting to RabbitMQ...")
	l.AMQP, err = NewMQConnection(cfg, l.Db)
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
	l.Db, err = NewDBConnection(cfg)
	fail(err)

	return l
}

func S(name string) *Londo {
	l := &Londo{
		Name: name,
	}

	log.Info("Starting " + l.Name + " service...")

	return l
}

func (l *Londo) GRPCServer() *Londo {
	log.Info("initializing grpc...")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	fail(err)

	opts := []grpc.ServerOption{
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_auth.StreamServerInterceptor(AuthIntercept),
		)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_auth.UnaryServerInterceptor(AuthIntercept),
		)),
	}

	srv := grpc.NewServer(opts...)
	londopb.RegisterCertServiceServer(srv, &GRPCServer{
		Londo: l,
	})

	reflection.Register(srv)

	go func() {
		log.Infof("server is running on port %d", cfg.GRPC.Port)
		if err := srv.Serve(lis); err != nil {
			fail(err)
		}
	}()

	return l
}

func (l *Londo) Run() error {
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)

	<-s
	log.Info("Goodbye, Captain Sheridan!")
	l.shutdown(0)

	return nil
}

func (l *Londo) RestAPIClient() *Londo {
	l.RestClient = NewRestClient(cfg)
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
