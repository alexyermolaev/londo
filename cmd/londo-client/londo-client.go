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
	version = "0.2.2"
)

var (
	app *cli.App
)

func init() {
	token := londocli.NewToken()
	server := londocli.NewServer()
	certPath := londocli.NewCertPath()

	app = cli.NewApp()

	app.Name = name
	app.Usage = usage
	app.Version = version
	app.Copyright = londocli.GetCopyright()
	app.Authors = []cli.Author{londocli.GetAuthors()}

	app.Commands = []cli.Command{
		cli.Command{
			Name:    "get",
			Aliases: []string{"g"},
			Usage:   "retrieves all corelated certificates and private keys from the remote",
			Action:  londocli.CallCertService,
		},
		cli.Command{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "request an updated token from remote host",
			Action:  londocli.UpdateToken,
		}}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "token, t",
			Usage:       "load token from `FILE`",
			Destination: &token.File,
			FilePath:    "config/token",
			EnvVar:      "LONDO_TOKEN",
		},
		cli.StringFlag{
			Name:        "server, s",
			Usage:       "connect to server `SERVER:PORT`",
			Destination: &server.String,
			Value:       "127.0.0.1:1337",
			EnvVar:      "LONDO_SERVER",
		},
		cli.StringFlag{
			Name:        "public, p",
			Usage:       "full `PATH` to public certificates directory",
			Destination: &certPath.Public,
			Value:       "/etc/pki/tls/certs/",
			EnvVar:      "LONDO_CERT_PUBLIC",
		},
		cli.StringFlag{
			Name:        "key, k",
			Usage:       "full `PATH` to private key directory",
			Destination: &certPath.Private,
			Value:       "/etc/pki/tls/private",
			EnvVar:      "LONDO_CERT_PRIVATE",
		},
	}

	app.EnableBashCompletion = true

	sort.Sort(cli.CommandsByName(app.Commands))
	sort.Sort(cli.FlagsByName(app.Flags))
}

func main() {
	app.Run(os.Args)
}
