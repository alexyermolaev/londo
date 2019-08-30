package londo

import (
	"context"
	"fmt"
	"sync"

	"github.com/alexyermolaev/londo/jwt"
	"github.com/alexyermolaev/londo/londopb"
	grpcauth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	Londo *Londo
}

func (g *GRPCServer) GetToken(ctx context.Context, req *londopb.GetTokenRequest) (*londopb.GetTokenResponse, error) {
	ip, _, err := ParseIPAddr(ctx)
	if err != nil {

		log.WithFields(logrus.Fields{logData: "ip"}).Error(err)

		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprint("server error"))
	}

	token, err := jwt.IssueJWT(ip)
	if err != nil {
		log.WithFields(logrus.Fields{logData: "jwt"}).Error(err)
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprint("server error"))
	}

	log.WithFields(logrus.Fields{logIP: ip}).Info("update token")
	return &londopb.GetTokenResponse{
		Token: &londopb.JWTToken{
			Token: string(token),
		},
	}, nil
}

func (g *GRPCServer) RenewSubjects(
	req *londopb.RenewSubjectRequest, stream londopb.CertService_RenewSubjectsServer) error {

	s := req.GetSubject()

	sr, err := g.setupRequest(stream.Context())
	if err != nil {
		return err
	}

	if s != "" {
		log.Infof("%s: renew %s", sr.ip, s)
		log.Infof("%s: sub %s -> queue %s", sr.ip, s, sr.addr)
		if err := g.Londo.Publish(
			DbReplyExchange,
			DbReplyQueue,
			sr.addr,
			DbGetSubjectCmd,
			GetSubjectEvent{Subject: s},
		); err != nil {
			return err
		}

		rs := <-sr.replyChannel

		if rs.Subject == "" {
			log.Errorf("%s: code %d, resp %s", sr.ip, codes.NotFound, s)
			return status.Errorf(
				codes.NotFound,
				fmt.Sprintf("%s not found", s))
		}
		sr.wg.Wait()

		if err = g.Londo.Publish("", "", "", "", RenewEvent{
			ID:       rs.ID.Hex(),
			Subject:  rs.Subject,
			CertID:   rs.CertID,
			AltNames: rs.AltNames,
			Targets:  rs.Targets,
		}); err != nil {
			return err
		}
		log.Infof("%s: %s -> renew", sr.ip, s)

		res := &londopb.RenewResponse{
			Subject: &londopb.RenewSubject{
				Subject: s,
			},
		}

		return stream.Send(res)
	}

	// TODO: this method is incomplete

	return nil
}

func (g *GRPCServer) GetExpiringSubject(
	req *londopb.GetExpiringSubjectsRequest, stream londopb.CertService_GetExpiringSubjectServer) error {

	d := req.Days

	sr, err := g.setupRequest(stream.Context())
	if err != nil {
		return err
	}

	log.WithFields(logrus.Fields{logIP: sr.ip, logDays: d}).Info("get expiring subjects")

	if err := g.Londo.Publish(
		DbReplyExchange,
		DbReplyQueue,
		sr.addr,
		DbGetExpiringSubjectsCmd,
		GetExpiringSubjEvent{Days: d},
	); err != nil {
		return err
	}

	return g.getManyReplies(sr, func(rs Subject) error {
		return stream.Send(&londopb.GetExpringSubjectsResponse{
			Subject: &londopb.ExpiringSubject{
				Subject: rs.Subject,
				ExpDate: rs.NotAfter.Unix(),
			},
		})
	})
}

func (g *GRPCServer) DeleteSubject(
	ctx context.Context, req *londopb.DeleteSubjectRequest) (*londopb.DeleteSubjectResponse, error) {
	s := req.GetSubject()

	ip, _, err := ParseIPAddr(ctx)
	if err != nil {
		return nil, err
	}

	log.WithFields(logrus.Fields{logIP: ip, logSubject: s}).Info("revoke")

	return nil, err
}

func (g *GRPCServer) AddNewSubject(
	ctx context.Context, req *londopb.AddNewSubjectRequest) (*londopb.AddNewSubjectResponse, error) {

	s := req.GetSubject().Subject
	subj := Subject{
		Subject:  s,
		Port:     req.GetSubject().Port,
		AltNames: req.GetSubject().AltNames,
		Targets:  req.GetSubject().Targets,
	}

	sr, err := g.setupRequest(ctx)
	if err != nil {
		return nil, err
	}

	log.WithFields(logrus.Fields{logIP: sr.ip, logSubject: s, logQueue: sr.addr}).Info("get")

	if err := g.Londo.Publish(
		DbReplyExchange, DbReplyQueue, sr.addr, DbGetSubjectCmd, GetSubjectEvent{Subject: s}); err != nil {
		return nil, err
	}

	rs := <-sr.replyChannel

	if rs.Subject != "" {

		log.WithFields(logrus.Fields{
			logIP:      sr.ip,
			logSubject: s,
			logCode:    codes.AlreadyExists}).Error("already exists")

		return nil, status.Errorf(
			codes.AlreadyExists,
			fmt.Sprintf("%s already exists", s))
	}

	sr.wg.Wait()

	if err = g.Londo.Publish(EnrollExchange, EnrollQueue, "", "", EnrollEvent{
		Subject:  subj.Subject,
		Port:     int(subj.Port),
		AltNames: subj.AltNames,
		Targets:  subj.Targets,
	}); err != nil {
		return nil, err
	}
	log.WithFields(logrus.Fields{logIP: sr.ip, logSubject: s, logQueue: EnrollQueue}).Info("enroll")

	return &londopb.AddNewSubjectResponse{
		Subject: s + " enrolled.",
	}, nil
}

func (g *GRPCServer) GetSubject(
	ctx context.Context, req *londopb.GetSubjectRequest) (*londopb.GetSubjectResponse, error) {

	s := req.GetSubject()

	sr, err := g.setupRequest(ctx)
	if err != nil {
		return nil, err
	}

	log.WithFields(logrus.Fields{logIP: sr.ip, logSubject: s, logQueue: sr.addr}).Info("get")

	if err := g.Londo.Publish(
		DbReplyExchange, DbReplyQueue, sr.addr, DbGetSubjectCmd, GetSubjectEvent{Subject: s}); err != nil {
		return nil, err
	}

	rs := <-sr.replyChannel

	if rs.Subject == "" {

		log.WithFields(logrus.Fields{logIP: sr.ip, logCode: codes.NotFound}).Error("not found")

		return nil, status.Errorf(
			codes.NotFound,
			fmt.Sprintf("%s not found", s))
	}

	sr.wg.Wait()

	log.Infof("%s: resp %s", sr.ip, rs.Subject)
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

	sr, err := g.setupRequest(stream.Context())
	if err != nil {
		return err
	}

	var targets []string
	targets = append(targets, sr.ip)

	log.WithFields(logrus.Fields{logIP: sr.ip, logQueue: sr.addr}).Info("get all")

	if err := g.Londo.Publish(
		DbReplyExchange,
		DbReplyQueue,
		sr.addr,
		DbGetSubjectByTargetCmd,
		GetSubjectByTargetEvent{Target: targets},
	); err != nil {
		return err
	}

	return g.getManyReplies(sr, func(rs Subject) error {
		return stream.Send(&londopb.GetSubjectResponse{
			Subject: &londopb.Subject{
				Subject:     rs.Subject,
				Certificate: rs.Certificate,
				PrivateKey:  rs.PrivateKey,
				AltNames:    rs.AltNames,
				Targets:     rs.Targets,
			},
		})
	})

}

func (g *GRPCServer) GetSubjectsByTarget(
	req *londopb.TargetRequest, stream londopb.CertService_GetSubjectsByTargetServer) error {

	targets := req.GetTarget()

	sr, err := g.setupRequest(stream.Context())
	if err != nil {
		return err
	}

	log.Infof("%s: subj by ip -> %s queue", sr.ip, sr.addr)
	if err := g.Londo.Publish(
		DbReplyExchange,
		DbReplyQueue,
		sr.addr,
		DbGetSubjectByTargetCmd,
		GetSubjectByTargetEvent{Target: targets},
	); err != nil {
		return err
	}

	return g.getManyReplies(sr, func(rs Subject) error {
		return stream.Send(&londopb.GetSubjectResponse{
			Subject: &londopb.Subject{
				Subject:     rs.Subject,
				Certificate: rs.Certificate,
				PrivateKey:  rs.PrivateKey,
				AltNames:    rs.AltNames,
				Targets:     rs.Targets,
			},
		})
	})
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

type requestSetup struct {
	replyChannel chan Subject
	doneChannel  chan struct{}
	wg           sync.WaitGroup
	ip           string
	addr         string
}

func (g *GRPCServer) setupRequest(ctx context.Context) (*requestSetup, error) {
	ip, addr, err := ParseIPAddr(ctx)
	if err != nil {
		return nil, err
	}

	rs := &requestSetup{
		replyChannel: make(chan Subject),
		doneChannel:  make(chan struct{}),
		ip:           ip,
		addr:         addr,
	}

	if err := g.Londo.DeclareBindQueue(GRPCServerExchange, rs.addr); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprint("server error"))
	}

	rs.wg.Add(1)

	g.Londo.ConsumeGrpcReplies(addr, rs.replyChannel, rs.doneChannel, &rs.wg)

	return rs, nil
}

func (g *GRPCServer) getManyReplies(sr *requestSetup, f func(rs Subject) error) error {
	for {
		select {
		case rs := <-sr.replyChannel:
			if rs.Subject == "" {
				log.Errorf("%s: code %d", sr.ip, codes.NotFound)

				<-sr.doneChannel
				sr.wg.Wait()
				return status.Errorf(
					codes.NotFound,
					fmt.Sprintf("no subjects found"))
			}

			if err := f(rs); err != nil {
				return err
			}

		case _ = <-sr.doneChannel:
			sr.wg.Wait()
			return nil
		}
	}
}
