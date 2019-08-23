package main

import "github.com/alexyermolaev/londo"

func main() {

	londo.S("gRPC server").
		AMQPConnection().
		GRPCServer().
		Run()
}
