package londo

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
