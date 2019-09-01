package logger

import (
	"github.com/sirupsen/logrus"
)

const (
	Subject  = "subject"
	IP       = "ip"
	Exchange = "exchange"
	Queue    = "queue"
	Cmd      = "cmd"
	Data     = "data"
	Days     = "days"
	Code     = "code"
	Target   = "target"
	Outdated = "outdated"
	Targets  = "targets"
	CertID   = "cert_id"
	Reason   = "reason"
	Level    = "level"
	Name     = "name"
	Port     = "port"
	Action   = "action"
	Count    = "count"
	Hours    = "hours"
	Minutes  = "minutes"
	Serial   = "remote_serial"
	DbSerial = "db_serial"
	Service  = "service"

	Requeue   = "requeue"
	Rejected  = "rejected"
	Ack       = "acknowledged"
	Published = "published"
	Received  = "received"
	Success   = "success"
	Skip      = "skipping"
)

func Fail(err error) {
	if err != nil {
		logrus.WithFields(logrus.Fields{Reason: err}).Fatal("crash")
	}
}
