package main

import (
	"os"
	"sort"

	londocli "github.com/alexyermolaev/londo/cli"
	"github.com/urfave/cli"
)

const (
	name    = "londo-admin"
	usage   = "A command line interface, allows interraction with Londo Certificate Management"
	version = "0.2.0"
)

var (
	// Commands
	tokenCmd = cli.Command{
		Name:    "token",
		Aliases: []string{"t"},
		Usage:   "issue new token for a target (i.e. IP address) system. it has to match a certificate subject target",
		Action:  londocli.IssueToken,
	}

	tgtCmd = cli.Command{
		Name:    "target",
		Aliases: []string{"tt"},
		Usage:   "get all subjects for distribution target",
		Action:  londocli.GetForTarget,
	}

	subjCmd = cli.Command{
		Name:    "subject",
		Aliases: []string{"s"},
		Usage:   "provides functionality to add, view and delete subjects",
		Subcommands: []cli.Command{
			addSubjCmd,
			delSubjCmd,
			getSubjCmd,
		},
	}

	addSubjCmd = cli.Command{
		Name:        "add",
		Aliases:     []string{"a"},
		Usage:       "add subject",
		Description: "add new subject, its alternative names and distribution targets",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name: "alt, a",
			},
			cli.StringSliceFlag{
				Name: "target, t",
			},
		},
		Action: londocli.AddSubject,
	}

	delSubjCmd = cli.Command{
		Name:        "delete",
		Aliases:     []string{"d", "del"},
		Usage:       "delete subject",
		Description: "schedules an existing subject to be removed, and its certificate revoked",
		Action:      londocli.DeleteSubject,
	}

	getSubjCmd = cli.Command{
		Name:    "get",
		Aliases: []string{"g"},
		Usage:   "get a subject",
		Action:  londocli.GetSubject,
	}

	app *cli.App

	argErr = cli.NewExitError("must specify an argument", 1)

	err error
)

func init() {
	server := londocli.NewServer()
	_ = londocli.NewToken()

	app = cli.NewApp()

	app.Name = name
	app.Usage = usage
	app.Version = version
	app.Copyright = londocli.GetCopyright()
	app.Authors = []cli.Author{londocli.GetAuthors()}

	app.Commands = []cli.Command{subjCmd, tokenCmd, tgtCmd}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "server, s",
			Usage:       "connect to server `SERVER:PORT`",
			Destination: &server.String,
			Value:       "127.0.0.1:1337",
			EnvVar:      "LONDO_SERVER",
		},
	}

	app.EnableBashCompletion = true

	sort.Sort(cli.CommandsByName(app.Commands))
}

func main() {
	app.Run(os.Args)
}
