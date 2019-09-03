package main

import "github.com/alexyermolaev/londo/londov1"

func main() {
	app := londov1.Initialize("londo-test")
	app.AMQPService()
	app.DeclareAndBindQueue("enroll")
	app.Run()
}
