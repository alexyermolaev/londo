package main

import (
	"os"
	"sort"

	"github.com/alexyermolaev/londo"
	londocli "github.com/alexyermolaev/londo/cli"
	"github.com/streadway/amqp"
	"github.com/urfave/cli"
)

const (
	name  = "londo-checkerd"
	usage = "checks DNS records of all subjects"
)

var (
	app *cli.App
)

func init() {
	app = londocli.DaemonSetup(name, usage, defaultCommand)

	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:        "hours",
			Usage:       "specify number of `HOURS` between checks",
			Value:       12,
			Destination: &londo.ScanHours,
		},
	}

	for _, f := range londo.DefaultFlags {
		app.Flags = append(app.Flags, f)
	}

	sort.Sort(cli.FlagsByName(app.Flags))
}

/*
Publishes a message with a request to get all subjects.Upon receiving a response, the daemon
checks if outdated date is older than set time. If a subject still cannot be resolved asks
database to delete a record, and publishes a message to revoke outdated and unresolvable subject.

If a subject was current, but cannot be resolved now, checker publishes a message to update database
with time and date, when subject became unresolvable.
*/
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
		AMQPConnection().
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect, nil).
		Declare(
			londo.CheckExchange,
			londo.CheckQueue,
			amqp.ExchangeDirect, nil).
		PublishPeriodicly(1).
		ConsumeCheck().
		Run()
}
