package main

import (
	"os"
	"sort"

	londocli "github.com/alexyermolaev/londo/cli"
	"github.com/urfave/cli"
)

const (
	name    = "Londo Client"
	usage   = "A client that allows interraction with Londo Certificate Management"
	version = "0.2.0"
)

var (
	subjCmd = cli.Command{
		Name:    "get",
		Aliases: []string{"g"},
		Usage:   "retrieves all corelated certificates and private keys from the remote",
		Action:  londocli.CallCertService,
	}

	updCmd = cli.Command{
		Name:    "update",
		Aliases: []string{"u"},
		Usage:   "updates and token from remote host",
		Action:  londocli.UpdateToken,
	}

	app *cli.App
)

func init() {
	token := londocli.NewToken()
	server := londocli.NewServer()

	app = cli.NewApp()

	app.Name = name
	app.Usage = usage
	app.Version = version
	app.Copyright = londocli.GetCopyright()
	app.Authors = []cli.Author{londocli.GetAuthors()}

	app.Commands = []cli.Command{subjCmd, updCmd}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "token, t",
			Usage:       "load token from `FILE`",
			Destination: &token.File,
			FilePath:    "config/token",
			Required:    true,
		},
		cli.StringFlag{
			Name:        "server, s",
			Usage:       "connect to server `SERVER:PORT`",
			Destination: &server.String,
			Value:       "127.0.0.1:1337",
		},
	}

	app.EnableBashCompletion = true

	sort.Sort(cli.CommandsByName(app.Commands))
	sort.Sort(cli.FlagsByName(app.Flags))
}

func main() {
	app.Run(os.Args)
}
