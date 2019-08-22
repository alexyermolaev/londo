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

type Log struct {
	Info  chan string
	Warn  chan string
	Debug chan string
	Err   chan error
	// Third trimester vacuum cleaning!
	Abort chan error
}

func CreateLogChannel() *Log {
	return &Log{
		Info:  make(chan string),
		Warn:  make(chan string),
		Err:   make(chan error),
		Abort: make(chan error),
	}
}
