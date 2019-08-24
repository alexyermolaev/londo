package londo

import (
	"context"
	"net"

	"github.com/alexyermolaev/londo/londopb"
	log "github.com/sirupsen/logrus"
)

type GRPCServer struct {
	Londo *Londo
}

func (g *GRPCServer) GetSubject(ctx context.Context, req *londopb.GetSubjectRequest) (*londopb.GetSubjectResponse, error) {
	subj := Subject{Subject: req.Subject}
	p := g.getPeerAddr(ctx)

	log.Info("%s : GET subject %s, addr: %s", p.Addr.String(), subj)

	if err := g.Londo.DeclareBindQueue(GRPCServerExchange, p.Addr.String()); err != nil {
		return nil, err
	}

	log.Debug("creating consumer")
	ch := make(chan Subject)
	g.Londo.ConsumeGrpcReplies(GRPCServerExchange, ch)

	log.Debug("request %s", subj)
	g.Londo.PublishDbCommand(DbGetSubjectCommand, &subj, p.Addr.String())

	rs := <-ch

	log.Debug("%s : RESP %s", p.Addr.String(), rs.Subject)
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

type Peer struct {
	Addr net.Addr
}

type peerKey struct{}

func (g *GRPCServer) getPeerAddr(ctx context.Context) *Peer {
	return ctx.Value(peerKey{}).(*Peer)
}
