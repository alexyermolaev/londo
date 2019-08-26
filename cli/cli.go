package cli

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/alexyermolaev/londo"
	"github.com/alexyermolaev/londo/londopb"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
)

const (
	name      = "Londo"
	usage     = "A command line interface, allows interraction with Londo Certificate Management"
	version   = "0.1.0"
	copyright = "(c) 2019 Alex Yermolaev, MIT License"
)

var (
	token string

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
		Usage:   "provides functionality to add, view, delete and edit subjects",
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
		Action:  GetSubject,
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
	app.Copyright = copyright
	app.Authors = []cli.Author{authors}

	app.Commands = []cli.Command{subjCmd, tokenCmd}

	app.EnableBashCompletion = true

	sort.Sort(cli.CommandsByName(app.Commands))

	token, err = getToken()
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

func GetSubject(c *cli.Context) error {
	arg := c.Args().First()
	if arg == "" {
		return argErr
	}

	auth := &authCreds{
		token: token,
	}

	conn, err := grpc.Dial("127.0.0.1:1337",
		grpc.WithInsecure(),
		grpc.WithPerRPCCredentials(auth))
	if err != nil {
		return cli.NewExitError("unable to connect.", 127)
	}
	defer conn.Close()

	client := londopb.NewCertServiceClient(conn)

	req := &londopb.GetSubjectRequest{
		Subject: arg,
	}

	res, err := client.GetSubject(context.Background(), req)
	if err != nil {
		println(err.Error())
		return cli.NewExitError("bad response", 1)
	}

	fmt.Printf("cn: %s\n\n", res.Subject.Subject)

	fmt.Println("certificate:")
	fmt.Printf("%s\n", res.Subject.Certificate)

	fmt.Println("private key:")
	fmt.Printf("%s\n", res.Subject.PrivateKey)

	fmt.Print("alt names (DNSNames): ")
	for _, alt := range res.Subject.AltNames {
		fmt.Printf("%s ", alt)
	}
	fmt.Print("\n\n")

	fmt.Print("targets: ")
	for _, alt := range res.Subject.Targets {
		fmt.Printf("%s ", alt)
	}
	fmt.Print("\n\n")

	return nil
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

func getToken() (string, error) {
	p := "config/token"

	f, err := os.Open(p)
	defer f.Close()
	if err != nil {
		return "", err
	}

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}

	return strings.Trim(string(b), "\n"), nil
}
