package londo

import "github.com/sirupsen/logrus"

func CheckFatalError(err error) {
	if err != nil {
		logrus.Fatal(err)
	}
}

func ConfigureLogging(level logrus.Level) {
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}
