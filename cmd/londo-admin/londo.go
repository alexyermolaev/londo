package main

import (
	"os"

	"github.com/alexyermolaev/londo"
)

func main() {

	c, err := londo.ReadConfig()
	if err != nil {
		os.Exit(1)
	}

	token, err := londo.IssueJWT("0.0.0.0", c)
	if err != nil {
		os.Exit(1)
	}

	println(string(token))
}
