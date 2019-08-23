package londo

import (
	"context"

	"github.com/alexyermolaev/londo/londopb"
)

type GRPCServer struct{}

func (g GRPCServer) GetSubject(context.Context, *londopb.GetSubjectRequest) (*londopb.GetSubjectResponse, error) {
	panic("implement me")
}
