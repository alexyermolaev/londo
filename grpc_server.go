package londo

import (
	"context"
	"github.com/alexyermolaev/londo/londopb"
	"net"
)

type GRPCServer struct {
	Londo *Londo
}

func (g GRPCServer) GetSubject(ctx context.Context, req *londopb.GetSubjectRequest) (*londopb.GetSubjectResponse, error) {
	subj := Subject{Subject: req.Subject}
	p := g.getPeerAddr(ctx)

	if err := g.Londo.DeclareBindQueue(g.Londo.GRPCServerExchange(), p.Addr.String()); err != nil {
		return nil, err
	}

	Londo.PublishDbCommand(*g.Londo, g.Londo.DbGetSubjectCommand(), &subj, "")

	panic("implement me")
}

type Peer struct {
	Addr net.Addr
}

type peerKey struct{}

func (g GRPCServer) getPeerAddr(ctx context.Context) *Peer {
	return ctx.Value(peerKey{}).(*Peer)
}
