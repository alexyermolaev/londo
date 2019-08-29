package main

import (
	"os"
	"sort"

	"github.com/alexyermolaev/londo"
	londocli "github.com/alexyermolaev/londo/cli"
	"github.com/streadway/amqp"
	"github.com/urfave/cli"
)

// TODO: need to code scheduler for period collection of certificates

const (
	name  = "londo-collectd"
	usage = "collects enrolled certificates"
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

func defaultCommand(_ *cli.Context) error {
	return londo.S(name).
		AMQPConnection().
		RestAPIClient().
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect, nil).
		Declare(
			londo.CollectExchange,
			londo.CollectQueue,
			amqp.ExchangeDirect, nil).
		ConsumeCollect().
		Run()
}
