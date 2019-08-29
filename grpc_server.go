package londo

import (
	"context"
	"fmt"
	"sync"

	"github.com/alexyermolaev/londo/jwt"
	"github.com/alexyermolaev/londo/londopb"
	grpcauth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	Londo *Londo
}

func (g *GRPCServer) GetToken(ctx context.Context, req *londopb.GetTokenRequest) (*londopb.GetTokenResponse, error) {
	ip, _, err := ParseIPAddr(ctx)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprint("server error"))
	}

	log.Infof("%s: update token", ip)

	token, err := jwt.IssueJWT(ip)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprint("server error"))
	}

	return &londopb.GetTokenResponse{
		Token: &londopb.JWTToken{
			Token: string(token),
		},
	}, nil
}

func (g *GRPCServer) RenewSubjects(
	req *londopb.RenewSubjectRequest, stream londopb.CertService_RenewSubjectsServer) error {

	s := req.GetSubject()

	ip, addr, err := ParseIPAddr(stream.Context())
	if err != nil {
		return err
	}

	if s != "" {
		log.Infof("%s: renew %s", ip, s)

		if err := g.Londo.DeclareBindQueue(GRPCServerExchange, addr); err != nil {
			log.Error(err)
			return status.Errorf(
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

		log.Infof("%s: sub %s -> queue %s", ip, s, addr)
		if err := g.Londo.Publish(
			DbReplyExchange,
			DbReplyQueue,
			addr,
			DbGetSubjectCmd,
			GetSubjectEvent{Subject: s},
		); err != nil {
			return err
		}

		rs := <-ch

		if rs.Subject == "" {
			log.Errorf("%s: code %d, resp %s", ip, codes.NotFound, s)
			return status.Errorf(
				codes.NotFound,
				fmt.Sprintf("%s not found", s))
		}
		wg.Wait()

		if err = g.Londo.Publish("", "", "", "", RenewEvent{
			ID:       rs.ID.Hex(),
			Subject:  rs.Subject,
			CertID:   rs.CertID,
			AltNames: rs.AltNames,
			Targets:  rs.Targets,
		}); err != nil {
			return err
		}
		log.Infof("%s: %s -> renew", ip, s)

		res := &londopb.RenewResponse{
			Subject: &londopb.RenewSubject{
				Subject: s,
			},
		}

		stream.Send(res)

		return nil
	}
	return nil
}

func (g *GRPCServer) GetExpiringSubject(
	req *londopb.GetExpiringSubjectsRequest, stream londopb.CertService_GetExpiringSubjectServer) error {

	d := req.Days

	ip, addr, err := ParseIPAddr(stream.Context())
	if err != nil {
		return err
	}

	var (
		ch   = make(chan Subject)
		done = make(chan struct{})
		wg   sync.WaitGroup
	)

	if err := g.Londo.DeclareBindQueue(GRPCServerExchange, addr); err != nil {
		log.Error(err)
		return status.Errorf(
			codes.FailedPrecondition,
			fmt.Sprint("server error"))
	}

	log.Debug("creating consumer")

	wg.Add(1)
	g.Londo.ConsumeGrpcReplies(addr, ch, done, &wg)

	log.Infof("%s: get expiring subs", ip)
	if err := g.Londo.Publish(
		DbReplyExchange,
		DbReplyQueue,
		addr,
		DbGetExpiringSubjectsCmd,
		GetExpiringSubjEvent{Days: d},
	); err != nil {
		return err
	}
	log.Infof("%s: expiring subjects, %d days", ip, d)

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

			res := &londopb.GetExpringSubjectsResponse{
				Subject: &londopb.ExpiringSubject{
					Subject: rs.Subject,
					ExpDate: rs.NotAfter.Unix(),
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

func (g *GRPCServer) DeleteSubject(
	ctx context.Context, req *londopb.DeleteSubjectRequest) (*londopb.DeleteSubjectResponse, error) {
	s := req.GetSubject()

	ip, _, err := ParseIPAddr(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("%s: revoke %s", ip, s)

	return nil, err
}

func (g *GRPCServer) AddNewSubject(
	ctx context.Context, req *londopb.AddNewSubjectRequest) (*londopb.AddNewSubjectResponse, error) {

	s := req.GetSubject().Subject
	subj := Subject{
		Subject:  s,
		AltNames: req.GetSubject().AltNames,
		Targets:  req.GetSubject().Targets,
	}

	var (
		ch = make(chan Subject)
		wg sync.WaitGroup
	)

	ip, addr, err := ParseIPAddr(ctx)
	if err != nil {
		return nil, err
	}

	if err := g.Londo.DeclareBindQueue(GRPCServerExchange, addr); err != nil {
		return nil, status.Errorf(
			codes.FailedPrecondition,
			fmt.Sprint("server error"))
	}

	log.Debug("creating consumer")

	wg.Add(1)
	g.Londo.ConsumeGrpcReplies(addr, ch, nil, &wg)

	log.Infof("%s: add %s", ip, s)
	log.Infof("%s: get %s -> queue %s", ip, s, addr)
	if err := g.Londo.Publish(
		DbReplyExchange, DbReplyQueue, addr, DbGetSubjectCmd, GetSubjectEvent{Subject: s}); err != nil {
		return nil, err
	}

	rs := <-ch

	if rs.Subject != "" {
		log.Errorf("%s: code %d, resp %s", ip, codes.AlreadyExists, s)
		return nil, status.Errorf(
			codes.AlreadyExists,
			fmt.Sprintf("%s already exists", s))
	}

	wg.Wait()

	if err = g.Londo.Publish(EnrollExchange, EnrollQueue, "", "", EnrollEvent{
		Subject:  subj.Subject,
		AltNames: subj.AltNames,
		Targets:  subj.Targets,
	}); err != nil {
		return nil, err
	}
	log.Infof("%s: %s -> enroll", ip, s)
	//g.Londo.PublishNewSubject(&subj)

	return &londopb.AddNewSubjectResponse{
		Subject: s + " has been queued for enrollment",
	}, nil
}

func (g *GRPCServer) GetSubject(
	ctx context.Context, req *londopb.GetSubjectRequest) (*londopb.GetSubjectResponse, error) {

	s := req.GetSubject()

	var (
		ch = make(chan Subject)
		wg sync.WaitGroup
	)

	ip, addr, err := ParseIPAddr(ctx)
	if err != nil {
		return nil, err
	}

	if err := g.Londo.DeclareBindQueue(GRPCServerExchange, addr); err != nil {
		return nil, status.Errorf(
			codes.FailedPrecondition,
			fmt.Sprint("server error"))
	}

	log.Debug("creating consumer")

	wg.Add(1)
	g.Londo.ConsumeGrpcReplies(addr, ch, nil, &wg)

	log.Infof("%s: get sub %s", ip, s)
	log.Infof("%s: sub %s -> queue %s", ip, s, addr)
	if err := g.Londo.Publish(
		DbReplyExchange, DbReplyQueue, addr, DbGetSubjectCmd, GetSubjectEvent{Subject: s}); err != nil {
		return nil, err
	}

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

func (g *GRPCServer) GetSubjectForTarget(
	req *londopb.ForTargetRequest, stream londopb.CertService_GetSubjectForTargetServer) error {

	ip, addr, err := ParseIPAddr(stream.Context())
	if err != nil {
		return err
	}

	var targets []string
	targets = append(targets, ip)

	log.Infof("%s: get subs by %s", ip, ip)

	return g.getSubjectsForIPAddr(targets, ip, addr, stream)
}

func (g *GRPCServer) GetSubjectsByTarget(
	req *londopb.TargetRequest, stream londopb.CertService_GetSubjectsByTargetServer) error {

	targets := req.GetTarget()

	ip, addr, err := ParseIPAddr(stream.Context())
	if err != nil {
		return err
	}

	log.Infof("%s: get subs by %s", ip, targets)

	return g.getSubjectsForIPAddr(targets, ip, addr, stream)
}

func (g *GRPCServer) getSubjectsForIPAddr(
	targets []string, ip string, addr string, stream londopb.CertService_GetSubjectsByTargetServer) error {

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
	if err := g.Londo.Publish(
		DbReplyExchange,
		DbReplyQueue,
		addr,
		DbGetSubjectByTargetCmd,
		GetSubjectByTargetEvent{Target: targets},
	); err != nil {
		return err
	}

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

	// For now we'll trust that if it's on localhost, it's ok to do whatever
	if ip == "127.0.0.1" {
		return ctx, nil
	}

	token, err := grpcauth.AuthFromMD(ctx, "bearer")
	if err != nil {
		log.Errorf("%s: no token present", ip)
		return nil, err
	}

	sub, err := jwt.VerifyJWT([]byte(token))
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
