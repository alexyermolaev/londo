package main

import (
	"os"
	"sort"

	londocli "github.com/alexyermolaev/londo/cli"
	"github.com/urfave/cli"
)

const (
	name    = "Londo"
	usage   = "A command line interface, allows interraction with Londo Certificate Management"
	version = "0.1.0"
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
		Name:    "add",
		Aliases: []string{"a"},
		Usage:   "add new subject",
	}

	delSubjCmd = cli.Command{
		Name:    "delete",
		Aliases: []string{"d", "del"},
		Usage:   "delete a subject",
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
	_ = londocli.NewToken()

	app = cli.NewApp()

	app.Name = name
	app.Usage = usage
	app.Version = version
	app.Copyright = londocli.GetCopyright()
	app.Authors = []cli.Author{londocli.GetAuthors()}

	app.Commands = []cli.Command{subjCmd, tokenCmd, tgtCmd}

	app.EnableBashCompletion = true

	sort.Sort(cli.CommandsByName(app.Commands))
}

func main() {
	app.Run(os.Args)
}
