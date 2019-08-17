package londo

import "github.com/sirupsen/logrus"

func CheckFatalError(err error) {
	if err != nil {
		logrus.Fatal(err)
	}
}

func LogConfig(level logrus.Level) {
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}
