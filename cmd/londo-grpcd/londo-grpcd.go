package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexyermolaev/londo"
	"github.com/alexyermolaev/londo/londopb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func main() {

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 1337))
	if err != nil {
		log.Fatal(err)
	}

	log.Info("starting grpc server")
	var opts []grpc.ServerOption
	srv := grpc.NewServer(opts...)
	londopb.RegisterCertServiceServer(s, &londo.GRPCServer{})

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT)
	signal.Notify(c, syscall.SIGTERM)

	go func() {
		<-c
		log.Info("shutting down...")
		srv.GracefulStop()
	}()

	if err := srv.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
