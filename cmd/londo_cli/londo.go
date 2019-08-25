package main

import (
	"os"
	"sort"

	"github.com/alexyermolaev/londo"
	"github.com/urfave/cli"
)

const (
	name    = "Londo"
	usage   = "A command line interface, allows interraction with Londo Certificate Management"
	version = "0.1.0"
)

var (
	// Flags

	newToken = cli.Command{
		Name:  "token",
		Usage: "issue new token for a target (i.e. IP address) system. it has to match a certificate subject target",
	}

	addSubj = cli.Command{
		Name:  "add",
		Usage: "create a new subject and issue a certificate",
	}
)

func main() {

	c, err := londo.ReadConfig()
	if err != nil {
		os.Exit(1)
	}

	_, err = londo.IssueJWT("0.0.0.0", c)
	if err != nil {
		os.Exit(1)
	}

	app := cli.NewApp()

	app.Name = name
	app.Usage = usage
	app.Version = version

	app.Commands = []cli.Command{newToken, addSubj}

	sort.Sort(cli.CommandsByName(app.Commands))

	err = app.Run(os.Args)
	if err != nil {
		os.Exit(1)
	}
}
