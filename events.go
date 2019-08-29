package londo

import (
	"strconv"
	"time"

	"github.com/streadway/amqp"
)

type Event interface {
	GetMessage() amqp.Publishing
}

type RenewEvent struct {
	ID       string
	Subject  string
	CertID   int
	AltNames []string
	Targets  []string
}

func (e RenewEvent) GetMessage() amqp.Publishing {
	return amqp.Publishing{
		CorrelationId: e.ID,
		Expiration:    strconv.Itoa(int(time.Now().Add(1 * time.Minute).Unix())),
	}
}

type RevokeEvent struct {
	ID     string
	CertID int
}

func (RevokeEvent) GetMessage() amqp.Publishing {
	return amqp.Publishing{}
}

type EnrollEvent struct {
	Subject  string
	AltNames []string
	Targets  []string
}

func (EnrollEvent) GetMessage() amqp.Publishing {
	return amqp.Publishing{ContentType: ContentType}
}

type DeleteSubjEvent struct {
	CertID int
}

type CompleteEnrollEvent struct {
	CertID      int
	Certificate string
}

func (CompleteEnrollEvent) GetMessage() amqp.Publishing {
	return amqp.Publishing{}
}

type GetSubjectEvent struct {
	Subject string
}

func (GetSubjectEvent) GetMessage() amqp.Publishing {
	return amqp.Publishing{}
}

type GetSubjectByTargetEvent struct {
	Target []string
}

func (GetSubjectByTargetEvent) GetMessage() amqp.Publishing {
	return amqp.Publishing{}
}

type NewSubjectEvent struct {
	Subject    string
	CSR        string
	PrivateKey string
	CertID     int
	OrderID    string
	AltNames   []string
	Targets    []string
}

func (NewSubjectEvent) GetMessage() amqp.Publishing {
	return amqp.Publishing{}
}

type CollectEvent struct {
	CertID int
}

func (CollectEvent) GetMessage() amqp.Publishing {
	return amqp.Publishing{}
}

type GetExpiringSubjEvent struct {
	Days int32
}

func (GetExpiringSubjEvent) GetMessage() amqp.Publishing {
	return amqp.Publishing{
		Type: DbGetExpiringSubjectsCmd,
	}
}

type ExpiringSubjectEvent struct {
	Subject  string
	NotAfter time.Time
}

func (ExpiringSubjectEvent) GetMessage() amqp.Publishing {
	return amqp.Publishing{}
}

type CheckDNSEvent struct {
	Subject string
	Targets []string
	// TODO: it may not be possible to deserialize it and from JSON
	Unresolvable time.Time
}

func (CheckDNSEvent) GetMessage() amqp.Publishing {
	return amqp.Publishing{}
}
