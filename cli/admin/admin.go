package admin

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/alexyermolaev/londo"
	londocli "github.com/alexyermolaev/londo/cli"
	"github.com/urfave/cli"
)

const (
	name    = "Londo"
	usage   = "A command line interface, allows interraction with Londo Certificate Management"
	version = "0.1.0"
)

var (
	token string

	// Commands
	tokenCmd = cli.Command{
		Name:    "token",
		Aliases: []string{"t"},
		Usage:   "issue new token for a target (i.e. IP address) system. it has to match a certificate subject target",
		Action:  Token,
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
	app.Copyright = londocli.GetCopyright()
	app.Authors = []cli.Author{londocli.GetAuthors()}

	app.Commands = []cli.Command{subjCmd, tokenCmd, tgtCmd}

	app.EnableBashCompletion = true

	sort.Sort(cli.CommandsByName(app.Commands))

	token, err = londocli.GetToken()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type authCreds struct {
	token string
}

func (a *authCreds) RequireTransportSecurity() bool {
	return false
}

func (a *authCreds) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "bearer " + a.token,
	}, nil
}

func Token(c *cli.Context) error {
	arg := c.Args().First()

	if arg == "" {
		return argErr
	}

	cfg, err := londo.ReadConfig()
	if err != nil {
		return cli.NewExitError("cannot read config", 1)
	}

	t, err := londo.IssueJWT(arg, cfg)
	if err != nil {
		return cli.NewExitError("cannot issue a token", 1)
	}

	fmt.Printf("sub: %s token: %s\n", arg, string(t))
	return nil
}

func Run() error {
	return app.Run(os.Args)
}
