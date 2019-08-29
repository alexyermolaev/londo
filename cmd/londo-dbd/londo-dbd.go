package main

import (
	"os"
	"time"

	"github.com/alexyermolaev/londo"
	londocli "github.com/alexyermolaev/londo/cli"
	"github.com/streadway/amqp"
	"github.com/urfave/cli"
)

const (
	name  = "londo-dbd"
	usage = "database client"
)

var (
	app *cli.App
)

func init() {
	app = cli.NewApp()

	app.Name = name
	app.Usage = usage
	app.Version = londo.Version
	app.Copyright = londocli.Copyright
	app.Authors = []cli.Author{londocli.GetAuthors()}

	app.Flags = londo.DefaultFlags

	app.Action = defaultCommand
}

func main() {
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}

func defaultCommand(c *cli.Context) error {
	return londo.S(name).
		DbService().
		AMQPConnection().
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect, nil).
		Declare(
			londo.RenewExchange,
			londo.RenewQueue,
			amqp.ExchangeDirect, amqp.Table{
				"x-message-ttl": int(59 * time.Second / time.Millisecond),
			}).
		ConsumeDbRPC().
		Run()

}
