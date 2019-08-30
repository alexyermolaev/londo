package londo

const (
	logSubject  = "subject"
	logIP       = "ip"
	logExchange = "exchange"
	logQueue    = "queue"
	logCmd      = "cmd"
	logData     = "data"
	logDays     = "days"
	logCode     = "code"
	logTarget   = "target"
	logTargets  = "targets"
	logCertID   = "cert_id"
	logReason   = "reason"
)

func fail(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
