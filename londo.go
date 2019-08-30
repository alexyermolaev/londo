package londo

import (
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/alexyermolaev/londo/londopb"
	grpcMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpcAuth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"github.com/urfave/cli"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
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
	DbDeleteSubjCmd          = "subj.delete"
	DbAddSubjCmd             = "subj.add"
	DbUpdateSubjCmd          = "subj.update"
	DbGetSubjectCmd          = "subj.get"
	DbGetAllSubjectsCmd      = "subj.get.all"
	DbGetSubjectByTargetCmd  = "subj.get.target"
	DbGetExpiringSubjectsCmd = "subj.get.expiring"
	DbUpdateCertStatusCmd    = "subj.update.status"

	// Tell consumer to close channel
	CloseChannelCmd = "stop"

	ContentType = "application/json"

	Version = "0.1.0"
)

var (
	Debug     bool
	ScanHours int

	cfg *Config
	err error

	log = logrus.New()

	DefaultFlags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug, d",
			Usage:       "enables debug level logging",
			EnvVar:      "LONDO_DEBUG",
			Destination: &Debug,
		},
		cli.StringFlag{
			Name:   "config, c",
			Usage:  "load configuration from `FILE`",
			EnvVar: "LONDO_CONFIG",
		},
	}
)

func init() {
	// Logging
	log.SetFormatter(&prefixed.TextFormatter{
		FullTimestamp:    true,
		SpacePadding:     20,
		DisableUppercase: true,
	})

}

type Londo struct {
	Name       string
	Db         *MongoDB
	AMQP       *AMQP
	GRPC       *GRPCServer
	RestClient *RestAPI
}

func (l *Londo) AMQPConnection() *Londo {

	log.WithFields(logrus.Fields{logIP: cfg.AMQP.Hostname, logPort: cfg.AMQP.Port}).Info("amqp connection")
	l.AMQP, err = NewMQConnection(cfg, l.Db)
	fail(err)

	return l
}

func (l *Londo) Declare(exchange string, queue string, kind string, args amqp.Table) *Londo {
	ch, err := l.AMQP.connection.Channel()
	defer ch.Close()
	fail(err)

	log.WithFields(logrus.Fields{logExchange: exchange}).Info("declaring")
	err = ch.ExchangeDeclare(
		exchange, kind, true, false, false, false, nil)
	fail(err)

	log.WithFields(logrus.Fields{logQueue: queue}).Info("declaring")
	q, err := ch.QueueDeclare(
		queue, false, false, false, false, args)
	fail(err)

	log.WithFields(logrus.Fields{logQueue: q.Name}).Info("binding")
	err = ch.QueueBind(queue, queue, exchange, false, nil)
	fail(err)

	return l
}

// TODO: refactor
func (l *Londo) DeclareExchange(exchange string, kind string) *Londo {
	ch, err := l.AMQP.connection.Channel()
	defer ch.Close()
	fail(err)

	log.WithFields(logrus.Fields{logExchange: exchange}).Info("declaring")
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

	log.WithFields(logrus.Fields{logQueue: queue}).Info("declaring")
	q, err := ch.QueueDeclare(queue, false, true, false, false, nil)
	if err != nil {
		return err
	}

	log.WithFields(logrus.Fields{logQueue: q.Name}).Info("binding")
	err = ch.QueueBind(queue, queue, exchange, false, nil)

	return err
}

func (l *Londo) DbService() *Londo {
	var err error

	log.WithFields(logrus.Fields{logIP: cfg.DB.Hostname, logPort: cfg.DB.Port}).Info("db connection")
	l.Db, err = NewDBConnection(cfg)
	fail(err)

	return l
}

func S(name string) *Londo {
	l := &Londo{
		Name: name,
	}

	if Debug {
		log.SetLevel(logrus.DebugLevel)
		log.WithFields(logrus.Fields{logLevel: "debug"}).Warn("logging")
	} else {
		log.SetLevel(logrus.InfoLevel)
		log.WithFields(logrus.Fields{logLevel: "info"}).Info("logging")
	}

	log.Info("reading config")
	cfg, err = ReadConfig()
	if err != nil {
		log.WithFields(logrus.Fields{logReason: err}).Error("cannot read config file")
		os.Exit(1)
	}

	log.WithFields(logrus.Fields{logName: l.Name}).Info("initializing")

	return l
}

func (l *Londo) GRPCServer() *Londo {

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	fail(err)

	opts := []grpc.ServerOption{
		grpc.StreamInterceptor(grpcMiddleware.ChainStreamServer(
			grpcAuth.StreamServerInterceptor(AuthIntercept),
		)),
		grpc.UnaryInterceptor(grpcMiddleware.ChainUnaryServer(
			grpcAuth.UnaryServerInterceptor(AuthIntercept),
		)),
	}

	srv := grpc.NewServer(opts...)
	londopb.RegisterCertServiceServer(srv, &GRPCServer{
		Londo: l,
	})

	reflection.Register(srv)

	go func() {
		log.WithFields(logrus.Fields{logPort: cfg.GRPC.Port, logIP: "0.0.0.0"}).Info("ready")
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
