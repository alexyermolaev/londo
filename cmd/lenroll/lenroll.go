package main

import "github.com/alexyermolaev/londo"

func main() {
	londo.S("enroll").
		RabbitMQService().
		EventService(londo.EnrollEventName).
		Run()
}
