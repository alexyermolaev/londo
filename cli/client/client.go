package client

import (
	"os"
	"sort"

	londocli "github.com/alexyermolaev/londo/cli"
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
		Action:  londocli.GetForTarget,
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

func Run() error {
	return app.Run(os.Args)
}
