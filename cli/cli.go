package cli

import (
	"os"
	"sort"

	"github.com/alexyermolaev/londo"
	"github.com/urfave/cli"
)

const (
	name      = "Londo"
	usage     = "A command line interface, allows interraction with Londo Certificate Management"
	version   = "0.1.0"
	copyright = "(c) 2019 Alex Yermolaev, MIT License"
)

var (
	authors = cli.Author{
		Name: "Alex Yermolaev",
	}

	// Commands
	tokenCmd = cli.Command{
		Name:    "token",
		Aliases: []string{"t"},
		Usage:   "issue new token for a target (i.e. IP address) system. it has to match a certificate subject target",
		Action:  Token,
	}

	subjCmd = cli.Command{
		Name:    "subject",
		Aliases: []string{"s"},
		Usage:   "subject management",
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
	}

	app *cli.App
)

func init() {

	c, err := londo.ReadConfig()
	if err != nil {
		os.Exit(1)
	}

	_, err = londo.IssueJWT("0.0.0.0", c)
	if err != nil {
		os.Exit(1)
	}

	app = cli.NewApp()

	app.Name = name
	app.Usage = usage
	app.Version = version
	app.Copyright = copyright
	app.Authors = []cli.Author{authors}

	app.Commands = []cli.Command{subjCmd, tokenCmd}

	app.EnableBashCompletion = true

	sort.Sort(cli.CommandsByName(app.Commands))

}

func Token(c *cli.Context) error {
	println(c.Args().First())
	return nil
}

func Run() error {
	return app.Run(os.Args)
}
