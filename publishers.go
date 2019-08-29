package londo

import (
	"encoding/json"
)

func (l *Londo) Publish(exchange string, queue string, e Event, reply string, cmd string) error {
	msg := e.GetMessage()
	if reply != "" {
		msg.ReplyTo = reply
	}

	if cmd != "" {
		msg.Type = cmd
	}

	msg.Body, err = json.Marshal(&e)
	if err != nil {
		return err
	}

	if err := l.AMQP.Emit(exchange, queue, msg); err != nil {
		return err
	}

	return nil
}
