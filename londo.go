package londo

import (
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/alexyermolaev/londo/jwt"
	"github.com/alexyermolaev/londo/logger"
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

	RevokeExchange = "revoke-rpc"
	RevokeQueue    = "revoke"

	CollectExchange = "collect-rpc"
	CollectQueue    = "collect"

	CheckExchange = "check-rpc"
	CheckQueue    = "check"

	DbReplyExchange = "db-rpc"
	DbReplyQueue    = "db-rpc-replies"

	GRPCServerExchange = "grpc"

	// Commands
	// Db
	DbDeleteSubjCmd                = "subj.delete"
	DbAddSubjCmd                   = "subj.add"
	DbUpdateSubjCmd                = "subj.update"
	DbGetSubjectCmd                = "subj.get"
	DbGetAllSubjectsCmd            = "subj.get.all"
	DbGetSubjectByTargetCmd        = "subj.get.target"
	DbGetUpdatedSubjectByTargetCmd = "subj.get.update"
	DbGetExpiringSubjectsCmd       = "subj.get.expiring"
	DbUpdateCertStatusCmd          = "subj.update.status"

	// Tell consumer to close channel
	CloseChannelCmd = "stop"

	ContentType = "application/json"

	Version = "0.1.0"
)

var (
	Debug       bool
	ScanHours   int
	RevokeHours int
	cfgFile     string

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
			Name:        "config, c",
			Usage:       "load configuration from `FILE`",
			EnvVar:      "LONDO_CONFIG",
			Destination: &cfgFile,
			Value:       "config/config.yml",
		},
	}
)

func init() {
	// Logging
	log.SetFormatter(&prefixed.TextFormatter{
		FullTimestamp:    true,
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
	l.AMQP, err = NewMQConnection(cfg, l.Db)
	Fail(err)

	log.WithFields(logrus.Fields{logger.Service: "amqp", logger.IP: cfg.AMQP.Hostname, logger.Port: cfg.AMQP.Port}).Info("connected")
	return l
}

func (l *Londo) Declare(exchange string, queue string, kind string, args amqp.Table) *Londo {
	ch, err := l.AMQP.connection.Channel()
	defer ch.Close()
	Fail(err)

	log.WithFields(logrus.Fields{logger.Exchange: exchange}).Info("declaring")
	err = ch.ExchangeDeclare(
		exchange, kind, true, false, false, false, nil)
	Fail(err)

	log.WithFields(logrus.Fields{logger.Queue: queue}).Info("declaring")
	q, err := ch.QueueDeclare(
		queue, false, false, false, false, args)
	Fail(err)

	log.WithFields(logrus.Fields{logger.Queue: q.Name}).Info("binding")
	err = ch.QueueBind(queue, queue, exchange, false, nil)
	Fail(err)

	return l
}

// TODO: refactor
func (l *Londo) DeclareExchange(exchange string, kind string) *Londo {
	ch, err := l.AMQP.connection.Channel()
	defer ch.Close()
	Fail(err)

	log.WithFields(logrus.Fields{logger.Exchange: exchange}).Info("declaring")
	err = ch.ExchangeDeclare(
		exchange, kind, true, false, false, false, nil)
	Fail(err)

	return l
}

// TODO: refactor
func (l *Londo) DeclareBindQueue(exchange string, queue string) error {
	ch, err := l.AMQP.connection.Channel()
	defer ch.Close()
	Fail(err)

	log.WithFields(logrus.Fields{logger.Queue: queue}).Info("declaring")
	q, err := ch.QueueDeclare(queue, false, true, false, false, nil)
	if err != nil {
		return err
	}

	log.WithFields(logrus.Fields{logger.Queue: q.Name}).Info("binding")
	err = ch.QueueBind(queue, queue, exchange, false, nil)

	return err
}

func (l *Londo) DbService() *Londo {
	var err error

	log.WithFields(logrus.Fields{logger.IP: cfg.DB.Hostname, logger.Port: cfg.DB.Port}).Info("db connection")
	l.Db, err = NewDBConnection(cfg)
	Fail(err)

	return l
}

func Initialize(name string) *Londo {
	l := &Londo{
		Name: name,
	}

	if Debug {
		log.SetLevel(logrus.DebugLevel)
		log.WithFields(logrus.Fields{logger.Level: "debug"}).Warn("logging")
	} else {
		log.SetLevel(logrus.InfoLevel)
		log.WithFields(logrus.Fields{logger.Level: "info"}).Info("logging")
	}

	log.Info("reading config")
	cfg, err = ReadConfig(cfgFile)
	Fail(err)

	log.WithFields(logrus.Fields{logger.Name: l.Name}).Info("initializing")

	return l
}

func (l *Londo) GRPCServer() *Londo {

	jwt.ReadSecret(SFile)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	Fail(err)

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
		log.WithFields(logrus.Fields{logger.Port: cfg.GRPC.Port, logger.IP: "0.0.0.0"}).Info("ready")
		if err := srv.Serve(lis); err != nil {
			Fail(err)
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

func Fail(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
