package londo

import log "github.com/sirupsen/logrus"

func fail(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
