package main

import (
	"github.com/alexyermolaev/londo"
	londocli "github.com/alexyermolaev/londo/cli"
	"github.com/streadway/amqp"
	"github.com/urfave/cli"
	"os"
	"sort"
)

const (
	name  = "londo-grpcd"
	usage = "allows admin and client interaction"
)

var (
	app *cli.App
)

func init() {
	app = londocli.DaemonSetup(name, usage, defaultCommand)

	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:        "port, p",
			Usage:       "`PORT` server listens on",
			EnvVar:      "LONDO_GRPC_PORT",
			Value:       56015,
			Destination: nil,
		},
		cli.StringFlag{
			Name:   "address, addr, a",
			Usage:  "interface `ADDRESS`",
			EnvVar: "LONDO_GRPC_ADDRESS",
			Value:  "0.0.0.0",
		},
		cli.BoolFlag{
			Name:   "insecure, i",
			Usage:  "disables port encryption (should not be used in production)",
			EnvVar: "LONDO_GRPC_INSECURE",
		},
	}

	for _, f := range londo.DefaultFlags {
		app.Flags = append(app.Flags, f)
	}

	app.Action = defaultCommand

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

	return londo.Initialize(name).
		AMQPConnection().
		Declare(
			londo.DbReplyExchange,
			londo.DbReplyQueue,
			amqp.ExchangeDirect, nil).
		Declare(
			londo.EnrollExchange,
			londo.EnrollQueue,
			amqp.ExchangeDirect, nil).
		DeclareExchange(
			londo.GRPCServerExchange,
			amqp.ExchangeDirect).
		GRPCServer().
		Run()
}
