package londo

import (
	"context"
	"fmt"

	"github.com/alexyermolaev/londo/londopb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	Londo *Londo
}

func (g *GRPCServer) GetSubject(ctx context.Context, req *londopb.GetSubjectRequest) (*londopb.GetSubjectResponse, error) {
	subj := Subject{Subject: req.Subject}

	ip, err := g.getIPAddress(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("%s : get subject %s", ip, req.Subject)

	if err := g.Londo.DeclareBindQueue(GRPCServerExchange, ip); err != nil {
		return nil, status.Errorf(
			codes.FailedPrecondition,
			fmt.Sprint("server error"))
	}

	log.Debug("creating consumer")
	ch := make(chan Subject)
	g.Londo.ConsumeGrpcReplies(ip, ch)

	log.Debugf("request %s", req.Subject)
	g.Londo.PublishDbCommand(DbGetSubjectCommand, &subj, ip)

	rs := <-ch

	if rs.Subject == "" {
		log.Errorf("%s : code %d, resp %s", ip, codes.NotFound, req.Subject)
		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("%s not found", req.Subject))
	}

	log.Infof("%s : resp %s", ip, rs.Subject)
	return &londopb.GetSubjectResponse{
		Subject: &londopb.Subject{
			Subject:     rs.Subject,
			Certificate: rs.Certificate,
			PrivateKey:  rs.PrivateKey,
			AltNames:    rs.AltNames,
			Targets:     rs.Targets,
		},
	}, nil
}

func (g *GRPCServer) getIPAddress(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", status.Errorf(
			codes.FailedPrecondition,
			fmt.Sprint("failed to get incoming ip address"))
	}
	return GetIPAddr(p.Addr.String()), nil
}
