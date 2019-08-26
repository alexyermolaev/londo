package londo

import (
	"context"
	"fmt"
	"sync"

	"github.com/alexyermolaev/londo/londopb"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	Londo *Londo
}

func (g *GRPCServer) AddNewSubject(ctx context.Context, req *londopb.AddNewSubjectRequest) (*londopb.GetSubjectResponse, error) {
	return nil, status.Errorf(
		codes.Unimplemented,
		fmt.Sprintf("not implemented"))
}

func (g *GRPCServer) GetSubject(ctx context.Context, req *londopb.GetSubjectRequest) (*londopb.GetSubjectResponse, error) {
	s := req.GetSubject()
	subj := Subject{Subject: s}

	ip, addr, err := ParseIPAddr(ctx)
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

	var (
		ch = make(chan Subject)
		wg sync.WaitGroup
	)

	wg.Add(1)
	g.Londo.ConsumeGrpcReplies(addr, ch, nil, &wg)

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

func (g *GRPCServer) GetSubjectForTarget(req *londopb.ForTargetRequest, stream londopb.CertService_GetSubjectForTargetServer) error {
	ip, addr, err := ParseIPAddr(stream.Context())
	if err != nil {
		return err
	}

	var targets []string
	targets = append(targets, ip)

	log.Infof("%s : get subjects by %s", ip, ip)

	return g.getSubjectsForIPAddr(targets, ip, addr, stream)
}

func (g *GRPCServer) GetSubjectsByTarget(req *londopb.TargetRequest, stream londopb.CertService_GetSubjectsByTargetServer) error {
	targets := req.GetTarget()

	ip, addr, err := ParseIPAddr(stream.Context())
	if err != nil {
		return err
	}

	log.Infof("%s : get subjects by %s", ip, targets)

	return g.getSubjectsForIPAddr(targets, ip, addr, stream)
}

func (g *GRPCServer) getSubjectsForIPAddr(targets []string, ip string, addr string, stream londopb.CertService_GetSubjectsByTargetServer) error {
	subj := Subject{Targets: targets}

	if err := g.Londo.DeclareBindQueue(GRPCServerExchange, addr); err != nil {
		return status.Errorf(
			codes.FailedPrecondition,
			fmt.Sprint("server error"))
	}

	log.Debug("creating consumer")

	var (
		ch   = make(chan Subject)
		done = make(chan struct{})
		wg   sync.WaitGroup
	)

	wg.Add(1)
	g.Londo.ConsumeGrpcReplies(addr, ch, done, &wg)

	log.Debugf("request subject by %s", targets)
	g.Londo.PublishDbCommand(DbGetSubjectByTargetCmd, &subj, addr)

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

func AuthIntercept(ctx context.Context) (context.Context, error) {
	ip, _, err := ParseIPAddr(ctx)
	if err != nil {
		log.Error("XXX.XXX.XXX.XXX: unable to parse incoming ip address")
		return nil, err
	}

	token, err := grpc_auth.AuthFromMD(ctx, "bearer")
	if err != nil {
		log.Debug("%s: no token present", ip)
		return nil, err
	}

	sub, err := VerifyJWT([]byte(token), cfg)
	if err != nil {
		log.Warnf("%s: authentication failed.", ip)
		return ctx, err
	}

	if sub != ip {
		log.Warnf("%s: != %s", ip, sub)
	}

	log.Info("connected")
	return ctx, nil
}
