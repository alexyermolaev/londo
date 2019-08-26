package cli

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/alexyermolaev/londo/londopb"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
)

const (
	copyright = "(c) 2019 Alex Yermolaev, MIT License"
)

var (
	authors = cli.Author{
		Name: "Alex Yermolaev",
	}

	argErr = cli.NewExitError("must specify an argument", 1)
)

func GetCopyright() string {
	return copyright
}

func GetAuthors() cli.Author {
	return authors
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

func GetForTarget(c *cli.Context) error {
	arg := c.Args().First()
	if arg == "" {
		return argErr
	}

	var token string

	auth := &authCreds{
		token: token,
	}

	// TODO: needs refactor
	conn, err := grpc.Dial("127.0.0.1:1337",
		grpc.WithInsecure(),
		grpc.WithPerRPCCredentials(auth))
	if err != nil {
		return cli.NewExitError("unable to connect.", 127)
	}
	defer conn.Close()

	client := londopb.NewCertServiceClient(conn)

	var targets []string
	targets = append(targets, arg)

	req := &londopb.TargetRequest{
		Target: targets,
	}

	stream, err := client.GetSubjectsByTarget(context.Background(), req)
	if err != nil {
		fmt.Println(err.Error())
		return cli.NewExitError("bad response", 1)
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("error while reading stream: %v", err)
			os.Exit(2)
		}
		fmt.Println(msg.GetSubject())
	}

	return nil
}

func GetSubject(c *cli.Context) error {
	arg := c.Args().First()
	if arg == "" {
		return argErr
	}

	var token string

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
		fmt.Println(err.Error())
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

// func Run() error {
// 	return app.Run(os.Args)
// }

func GetToken() (string, error) {
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
