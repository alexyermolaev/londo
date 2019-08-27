package client

import (
	"context"
	"os"
	"sort"

	londocli "github.com/alexyermolaev/londo/cli"
	"github.com/alexyermolaev/londo/londopb"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	name    = "Londo Client"
	usage   = "A client that allows interraction with Londo Certificate Management"
	version = "0.1.0"
)

var (
	subjCmd = cli.Command{
		Name:    "get",
		Aliases: []string{"g"},
		Usage:   "retrievs all corelated certificates and private keys from the remote",
		Action:  CallCertService,
	}

	app *cli.App
)

func init() {
	app = cli.NewApp()

	app.Name = name
	app.Usage = usage
	app.Version = version
	app.Copyright = londocli.GetCopyright()
	app.Authors = []cli.Author{londocli.GetAuthors()}

	app.Commands = []cli.Command{subjCmd}

	app.EnableBashCompletion = true

	sort.Sort(cli.CommandsByName(app.Commands))
}

func CallCertService(c *cli.Context) {
	UpdateToken(c)
	londocli.GetForTarget(c)
}

func UpdateToken(c *cli.Context) error {
	londocli.DoRequest(func(client londopb.CertServiceClient) error {
		req := &londopb.GetTokenRequest{}

		res, err := client.GetToken(context.Background(), req)
		if err != nil {
			log.Fatal(err)
		}

		log.Info(res.Token.Token)

		return nil
	})

	return nil
}

func Run() error {
	return app.Run(os.Args)
}
