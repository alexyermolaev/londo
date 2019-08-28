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
	name  = "londo-chckerd"
	usage = "checks DNS records of all subjects"
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

	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:        "hours",
			Usage:       "specify number of `HOURS` between checks",
			Value:       12,
			Destination: &londo.ScanHours,
		},
		// TODO: these are default flags all daemon share. They need to be elsewhere.
		cli.BoolFlag{
			Name:        "debug, d",
			Usage:       "turn `DEBUG` on and off",
			EnvVar:      "LONDO_DEBUG",
			Destination: &londo.Debug,
		},
		cli.StringFlag{
			Name:   "config, c",
			Usage:  "load configuration from `FILE`",
			EnvVar: "LONDO_CONFIG",
		},
	}

	app.Action = defaultCommand

	sort.Sort(cli.FlagsByName(app.Flags))
}

/*
Publishes a message with a request to get all subjects.Upon receiveing a response, the dameon
checks if outdated date is older than set time. If a subject still cannot be resolved asks
database to delete a record, and publshes a message to revoke outdated and unresolvable subject.

If a subject was current, but cannot be resolved now, checker publishes a message to update database
with time and date, when subject became unresolvable.
*/
func main() {
	app.Run(os.Args)
}

func defaultCommand(c *cli.Context) error {
	return londo.S(name).
		AMQPConnection().
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect, nil).
		ConsumeCheck().
		Run()
}
