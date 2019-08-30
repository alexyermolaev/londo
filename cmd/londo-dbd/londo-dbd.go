package main

import (
	"os"
	"sort"
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
	app = londocli.DaemonSetup(name, usage, defaultCommand)

	sort.Sort(cli.FlagsByName(app.Flags))
}

func main() {
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}

func defaultCommand(c *cli.Context) error {
	if c.Bool("debug") {
		londo.Debug = true
	}

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
