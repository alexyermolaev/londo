package londo

import (
	"context"
	"errors"
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
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("unable to get ip address from remote")
	}

	log.Infof("%s : get subject %s", p.Addr.String(), req.Subject)

	if err := g.Londo.DeclareBindQueue(GRPCServerExchange, p.Addr.String()); err != nil {
		return nil, err
	}

	log.Debug("creating consumer")
	ch := make(chan Subject)
	g.Londo.ConsumeGrpcReplies(p.Addr.String(), ch)

	log.Debugf("request %s", req.Subject)
	g.Londo.PublishDbCommand(DbGetSubjectCommand, &subj, p.Addr.String())

	rs := <-ch

	if rs.Subject == "" {
		log.Errorf("%s : code %d, resp %s", p.Addr.String(), codes.NotFound, req.Subject)
		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("%s not found", req.Subject))
	}

	log.Infof("%s : resp %s", p.Addr.String(), rs.Subject)
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
