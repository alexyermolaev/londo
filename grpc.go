package londo

import (
	"context"

	"github.com/alexyermolaev/londo/londopb"
)

type GRPCServer struct {
	Londo *Londo
}

func (g GRPCServer) GetSubject(ctx context.Context, req *londopb.GetSubjectRequest) (*londopb.GetSubjectResponse, error) {
	subj := Subject{Subject: req.Subject}

	Londo.PublishDbCommand(*g.Londo, g.Londo.DbGetSubjectCommand(), &subj, "")

	panic("implement me")
}
