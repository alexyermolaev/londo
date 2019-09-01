package londo

import (
	"context"
	"fmt"
	"sync"

	"github.com/alexyermolaev/londo/jwt"
	"github.com/alexyermolaev/londo/logger"
	"github.com/alexyermolaev/londo/londopb"
	grpcauth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	intError   = "internal error"
	notFound   = "not found"
	exists     = "already exists"
	noToken    = "no token"
	authFailed = "authentication failed"

	SFile string
)

type GRPCServer struct {
	Londo *Londo
}

func (g *GRPCServer) GetToken(ctx context.Context, req *londopb.GetTokenRequest) (*londopb.GetTokenResponse, error) {
	ip, _, err := ParseIPAddr(ctx)
	if err != nil {
		log.WithFields(logrus.Fields{logger.IP: logger.Unknown}).Error(err)

		return nil, internalError()
	}

	token, err := jwt.IssueJWT(ip)
	if err != nil {
		log.WithFields(logrus.Fields{logger.IP: ip}).Error(err)
		return nil, internalError()
	}

	log.WithFields(logrus.Fields{logger.IP: ip}).Info("update token")
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
		log.Error(err)
		return internalError()
	}

	if s != "" {
		fields := logrus.Fields{
			logger.Exchange: DbReplyExchange,
			logger.Queue:    DbReplyQueue,
			logger.Cmd:      DbGetSubjectCmd,
			logger.IP:       sr.ip,
			logger.Subject:  s}

		if err := g.Londo.Publish(
			DbReplyExchange,
			DbReplyQueue,
			sr.addr,
			DbGetSubjectCmd,
			GetSubjectEvent{Subject: s},
		); err != nil {
			log.WithFields(fields).Error(err)
			return notFoundError()
		}

		log.WithFields(fields).Info(logger.Get)

		rs := <-sr.replyChannel

		if rs.Subject == "" {
			log.WithFields(fields).Error(notFound)
			return notFoundError()
		}

		<-sr.doneChannel
		sr.wg.Wait()

		revEvent := RevokeEvent{
			ID:     rs.ID.Hex(),
			CertID: rs.CertID,
		}

		fields = logrus.Fields{
			logger.Exchange: RevokeExchange,
			logger.Queue:    RevokeQueue,
			logger.Subject:  rs.Subject,
			logger.CertID:   rs.CertID}

		if err = g.Londo.Publish(RevokeExchange, RevokeQueue, "", "", revEvent); err != nil {
			log.WithFields(fields).Error(err)
			return internalError()
		}

		log.WithFields(fields).Info(logger.Published)

		fields = logrus.Fields{
			logger.Exchange: EnrollExchange,
			logger.Queue:    EnrollQueue,
			logger.Subject:  rs.Subject,
			logger.CertID:   rs.CertID}

		if err := g.Londo.Publish(EnrollExchange, EnrollQueue, "", "", EnrollEvent{
			Subject:  rs.Subject,
			Port:     rs.Port,
			AltNames: rs.AltNames,
			Targets:  rs.Targets,
		}); err != nil {
			log.WithFields(fields).Error(err)
			return internalError()
		}

		log.WithFields(fields).Info(logger.Published)

		fields = logrus.Fields{
			logger.Exchange: DbReplyExchange,
			logger.Queue:    DbReplyQueue,
			logger.Subject:  rs.Subject,
			logger.CertID:   rs.CertID}

		if err := g.Londo.Publish(DbReplyExchange, DbReplyQueue, "", DbDeleteSubjCmd, revEvent); err != nil {
			log.WithFields(fields).Error(err)
			return internalError()
		}

		log.WithFields(fields).Info(logger.Published)
		res := &londopb.RenewResponse{Subject: &londopb.RenewSubject{Subject: s}}
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
		return internalError()
	}

	fields := logrus.Fields{
		logger.Exchange: DbReplyExchange,
		logger.Queue:    DbReplyQueue,
		logger.Reply:    sr.addr,
		logger.IP:       sr.ip,
		logger.Cmd:      DbGetExpiringSubjectsCmd,
	}

	if err := g.Londo.Publish(
		DbReplyExchange,
		DbReplyQueue,
		sr.addr,
		DbGetExpiringSubjectsCmd,
		GetExpiringSubjEvent{Days: d},
	); err != nil {
		log.WithFields(fields).Error(err)
		return internalError()
	}

	log.WithFields(fields).Info(logger.Published)

	return g.getManyReplies(sr, func(rs Subject) error {
		return stream.Send(&londopb.GetExpiringSubjectsResponse{
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

	log.WithFields(logrus.Fields{logger.IP: ip, logger.Subject: s}).Info(logger.Revoked)
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
		log.Error(err)
		return nil, internalError()
	}

	log.WithFields(logrus.Fields{logger.IP: sr.ip, logger.Subject: s, logger.Queue: sr.addr}).Info(logger.Get)

	fields := logrus.Fields{
		logger.Exchange: DbReplyExchange,
		logger.Queue:    DbReplyQueue,
		logger.IP:       sr.ip,
		logger.Cmd:      DbGetSubjectCmd,
	}

	if err := g.Londo.Publish(
		DbReplyExchange, DbReplyQueue, sr.addr, DbGetSubjectCmd, GetSubjectEvent{Subject: s}); err != nil {

		log.WithFields(fields).Error(err)
		return nil, internalError()
	}

	rs := <-sr.replyChannel

	if rs.Subject != "" {

		log.WithFields(logrus.Fields{
			logger.IP:      sr.ip,
			logger.Subject: s,
			logger.Code:    codes.AlreadyExists}).Error(exists)

		return nil, alreadyExistsError()
	}

	<-sr.doneChannel
	sr.wg.Wait()

	if err = g.Londo.Publish(EnrollExchange, EnrollQueue, "", "", EnrollEvent{
		Subject:  subj.Subject,
		Port:     subj.Port,
		AltNames: subj.AltNames,
		Targets:  subj.Targets,
	}); err != nil {
		return nil, err
	}
	log.WithFields(logrus.Fields{logger.IP: sr.ip, logger.Subject: s, logger.Queue: EnrollQueue}).Info(logger.Enroll)

	return &londopb.AddNewSubjectResponse{Subject: s + " enrolled."}, nil
}

func (g *GRPCServer) GetSubject(
	ctx context.Context, req *londopb.GetSubjectRequest) (*londopb.GetSubjectResponse, error) {

	s := req.GetSubject()

	sr, err := g.setupRequest(ctx)
	if err != nil {
		return nil, err
	}

	log.WithFields(logrus.Fields{logger.IP: sr.ip, logger.Subject: s, logger.Queue: sr.addr}).Info(logger.Get)

	if err := g.Londo.Publish(
		DbReplyExchange, DbReplyQueue, sr.addr, DbGetSubjectCmd, GetSubjectEvent{Subject: s}); err != nil {
		return nil, err
	}

	rs := <-sr.replyChannel

	if rs.Subject == "" {
		log.WithFields(logrus.Fields{logger.IP: sr.ip, logger.Code: codes.NotFound}).Error(notFound)
		return nil, notFoundError()
	}

	<-sr.doneChannel
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

	cmd := DbGetSubjectByTargetCmd
	if req.Update {
		cmd = DbGetUpdatedSubjectByTargetCmd
	}

	fields := logrus.Fields{
		logger.Exchange: DbReplyExchange,
		logger.Queue:    DbReplyQueue,
		logger.Reply:    sr.addr,
		logger.IP:       sr.ip,
		logger.Cmd:      cmd,
	}

	if err := g.Londo.Publish(
		DbReplyExchange,
		DbReplyQueue,
		sr.addr,
		cmd,
		GetSubjectByTargetEvent{Target: targets},
	); err != nil {
		log.WithFields(fields).Error(err)
		return internalError()
	}

	log.WithFields(fields).Info(logger.Published)

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

	fields := logrus.Fields{
		logger.Exchange: DbReplyExchange,
		logger.Queue:    DbReplyQueue,
		logger.Reply:    sr.addr,
		logger.IP:       sr.ip,
		logger.Cmd:      DbGetSubjectByTargetCmd,
	}

	if err := g.Londo.Publish(
		DbReplyExchange,
		DbReplyQueue,
		sr.addr,
		DbGetSubjectByTargetCmd,
		GetSubjectByTargetEvent{Target: targets},
	); err != nil {
		log.WithFields(fields).Error(err)
		return internalError()
	}

	log.WithFields(fields).Info(logger.Published)

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
		log.WithFields(logrus.Fields{logger.IP: logger.Unknown}).Error(err)
		return nil, invalidArgError()
	}

	fields := logrus.Fields{logger.IP: ip}

	// For now we'll trust that if it's on localhost, it's ok to do whatever
	if ip == "127.0.0.1" {
		return ctx, nil
	}

	token, err := grpcauth.AuthFromMD(ctx, "bearer")
	if err != nil {
		log.WithFields(fields).Error(noToken)
		return nil, invalidArgError()
	}

	sub, err := jwt.VerifyJWT([]byte(token))
	if err != nil {
		log.WithFields(fields).Error(authFailed)
		return nil, authFailedError()
	}

	if sub != ip {
		log.WithFields(fields).Error("!=")
		return nil, authFailedError()
	}

	log.WithFields(fields).Info(logger.Ok)
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
		return nil, internalError()
	}

	rs.wg.Add(1)

	g.Londo.ConsumeGRPCReplies(addr, rs.replyChannel, rs.doneChannel, &rs.wg)

	return rs, nil
}

func (g *GRPCServer) getManyReplies(sr *requestSetup, f func(rs Subject) error) error {
	for {
		select {
		case rs := <-sr.replyChannel:
			if rs.Subject == "" {
				log.WithFields(logrus.Fields{logger.IP: sr.ip}).Error(notFound)

				<-sr.doneChannel
				sr.wg.Wait()
				return notFoundError()
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

func internalError() error {
	return status.Errorf(codes.Internal, fmt.Sprintf(intError))
}

func notFoundError() error {
	return status.Errorf(codes.NotFound, fmt.Sprintf(notFound))
}

func alreadyExistsError() error {
	return status.Errorf(codes.AlreadyExists, fmt.Sprintf(exists))
}

func authFailedError() error {
	return status.Errorf(codes.Unauthenticated, fmt.Sprintf(authFailed))
}

func invalidArgError() error {
	return status.Errorf(codes.InvalidArgument, fmt.Sprintf(noToken))
}
