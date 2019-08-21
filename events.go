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

type DeleteSubjEvenet struct {
	CertID int
}

type CompleteEnrollEvent struct {
	Subject     string
	CertID      int
	OrderID     string
	Certificate string
}

type CSREvent struct {
	Subject    string
	CSR        string
	PrivateKey string
	AltNames   []string
	Targets    []string
}

type CollectEvent struct {
	CertID int
}
