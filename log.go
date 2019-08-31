package londo

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

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
	logOutdated = "outdated"
	logTargets  = "targets"
	logCertID   = "cert_id"
	logReason   = "reason"
	logLevel    = "level"
	logName     = "name"
	logPort     = "port"
	logAction   = "action"
	logCount    = "count"
	logHours    = "hours"
	logSerial   = "remote_serial"
	logDbSerial = "db_serial"
	logService  = "service"
)

func fail(err error) {
	if err != nil {
		log.WithFields(logrus.Fields{logReason: err}).Fatal("crash")
	}
}

func SetDebugLevel(c *cli.Context) {
}
