package main

import (
	"github.com/alexyermolaev/londo"
	log "github.com/sirupsen/logrus"
)

// lchecker checks expring certificates

func main() {
	londo.ConfigureLogging(log.DebugLevel)

	log.Info("Starting Database Daemon...")

	log.Info("Reading configuration")
	c, err := londo.ReadConfig()
	londo.CheckFatalError(err)

	db, err := londo.NewDBConnection(c)
	londo.CheckFatalError(err)
	log.Infof("Connecting to %v database", db.Name)

	exp, err := db.FindExpiringSubjects(720)
	londo.CheckFatalError(err)

	for _, e := range exp {
		log.Infof("%v, %v", e.Subject, e.NotAfter)
	}

	log.Infof("Disconnecting from the database")
	db.Disconnect()
}
