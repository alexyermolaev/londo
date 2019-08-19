package londo

type Event interface {
	EventName() string
}

type RenewEvent struct {
	Subject  string
	CertID   int
	AltNames []string
	Targets  []string
}

func (e RenewEvent) EventName() string {
	return RenewEventName
}

type RevokeEvent struct {
	CertID int
}

func (e RevokeEvent) EventName() string {
	return RevokeEventName
}

type EnrollEvent struct {
	Subject  string
	AltNames []string
	Targets  []string
}

func (e EnrollEvent) EventName() string {
	return EnrollEventName
}

type DeleteSubjEvenet struct {
	CertID int
}

func (e DeleteSubjEvenet) EventName() string {
	return DeleteSubjEvent
}

type CompleteEnrollEvent struct {
	Subject     string
	CertID      int
	OrderID     string
	Certificate string
}

func (e CompleteEnrollEvent) EventName() string {
	return CompleteEnrollName
}

type CSREvent struct {
	Subject    string
	CSR        string
	PrivateKey string
	AltNames   []string
	Targets    []string
}

func (e CSREvent) EventName() string {
	return CSREventName
}

type CollectEvent struct {
	CertID int
}

func (e CollectEvent) EventName() string {
	return CollectEventName
}
