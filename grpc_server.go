package londo

import (
	"context"
	"fmt"
	"sync"

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
	s := req.GetSubject()
	subj := Subject{Subject: s}

	ip, err := g.getIPAddress(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("%s: get subject %s", ip, s)

	if err := g.Londo.DeclareBindQueue(GRPCServerExchange, ip); err != nil {
		return nil, status.Errorf(
			codes.FailedPrecondition,
			fmt.Sprint("server error"))
	}

	log.Debug("creating consumer")
	ch := make(chan Subject)
	var wg sync.WaitGroup

	wg.Add(1)
	g.Londo.ConsumeGrpcReplies(ip, ch, nil, &wg)

	log.Debugf("request %s", s)
	g.Londo.PublishDbCommand(DbGetSubjectComd, &subj, ip)

	rs := <-ch

	if rs.Subject == "" {
		log.Errorf("%s: code %d, resp %s", ip, codes.NotFound, s)
		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("%s not found", s))
	}

	wg.Wait()

	log.Infof("%s: resp %s", ip, rs.Subject)
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

func (g *GRPCServer) GetSubjectsByTarget(req *londopb.TargetRequest, stream londopb.CertService_GetSubjectsByTargetServer) error {
	t := req.GetTarget()

	var tarr []string
	tarr = append(tarr, t)

	subj := Subject{Targets: tarr}
	ip, err := g.getIPAddress(stream.Context())
	if err != nil {
		return err
	}

	log.Infof("%s : get subjects by %s", ip, t)

	if err := g.Londo.DeclareBindQueue(GRPCServerExchange, ip); err != nil {
		return status.Errorf(
			codes.FailedPrecondition,
			fmt.Sprint("server error"))
	}

	log.Debug("creating consumer")
	ch := make(chan Subject)
	done := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(1)
	g.Londo.ConsumeGrpcReplies(ip, ch, done, &wg)

	log.Debugf("request subject by %s", t)
	g.Londo.PublishDbCommand(DbGetSubjectByTargetCmd, &subj, ip)

	for {
		select {
		case rs := <-ch:
			if rs.Subject == "" {
				log.Errorf("%s: code %d", ip, codes.NotFound)

				<-done
				wg.Wait()
				return status.Errorf(
					codes.NotFound,
					fmt.Sprintf("no subjects found"))
			}

			res := &londopb.GetSubjectResponse{
				Subject: &londopb.Subject{
					Subject:     rs.Subject,
					Certificate: rs.Certificate,
					PrivateKey:  rs.PrivateKey,
					AltNames:    rs.AltNames,
					Targets:     rs.Targets,
				},
			}
			stream.Send(res)

		case _ = <-done:
			wg.Wait()
			log.Infof("%s: close stream", ip)
			return nil
		}
	}
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
