package londo

import log "github.com/sirupsen/logrus"

func CheckFatalError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func ConfigureLogging(level log.Level) {
	log.SetLevel(level)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}
