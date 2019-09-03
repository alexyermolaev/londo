package logger

import (
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var (
	// Localization
	//
	// General
	File = "file"

	// AMQP
	Connection = "conn"
	Exchange   = "exchange"
	Queue      = "queue"

	// values
	Initializing  = "initializing"
	ReadingConf   = "reading config"
	Connecting    = "connecting"
	Connected     = "connected"
	Success       = "scccess"
	Declaring     = "declaring"
	Binding       = "binding"
	Unmarshalling = "unmarshalling"
	ShuttingDown  = "shutting down"
)

func SetupLogging(name string, debug bool) *logrus.Entry {

	l := logrus.New()

	l.SetFormatter(&prefixed.TextFormatter{
		FullTimestamp:    true,
		SpacePadding:     20,
		DisableUppercase: true,
	})

	if debug {
		l.SetLevel(logrus.DebugLevel)
	}

	return l.WithFields(logrus.Fields{"service": name})
}
