package londo

import "time"

type RenewEvent struct {
	Subject  string
	CertID   int
	AltNames []string
	Targets  []string
}

type RevokeEvent struct {
	CertID int
}

type EnrollEvent struct {
	Subject  string
	AltNames []string
	Targets  []string
}

type DeleteSubjEvent struct {
	CertID int
}

type CompleteEnrollEvent struct {
	CertID      int
	Certificate string
}

type GetSubjectEvenet struct {
	Subject string
}

type GetSubjectByTarget struct {
	Target []string
}

type NewSubjectEvenet struct {
	Subject    string
	CSR        string
	PrivateKey string
	CertID     int
	OrderID    string
	AltNames   []string
	Targets    []string
}

type CollectEvent struct {
	CertID int
}

type GetExpringSubjEvent struct {
	Days int32
}

type ExpiringSubjEventResponse struct {
	Subject  string
	NotAfter time.Time
}

type CheckDNSEvent struct {
	Subject string
	Targets []string
	// TODO: it may not be possible to deserialze it and from JSON
	Unresolvable time.Time
}
