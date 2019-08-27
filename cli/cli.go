package cli

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/alexyermolaev/londo/londopb"
	log "github.com/sirupsen/logrus"
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

	token string
	err   error
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	token, err = GetToken()
	if err != nil {
		fmt.Println("cannot read token")
		os.Exit(1)
	}
}

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

func GetForTarget(c *cli.Context) {
	arg := c.Args().First()
	var targets []string
	targets = append(targets, arg)

	prepareReq(func(client londopb.CertServiceClient) error {
		// TODO: need refactor
		if arg != "" {
			req := &londopb.TargetRequest{
				Target: targets,
			}

			stream, err := client.GetSubjectsByTarget(context.Background(), req)
			if err != nil {
				return cli.NewExitError(fmt.Sprintf("%v", err), 1)
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
		} else {
			req := &londopb.ForTargetRequest{}

			stream, err := client.GetSubjectForTarget(context.Background(), req)
			if err != nil {
				return cli.NewExitError(fmt.Sprintf("%v", err), 1)
			}

			for {
				msg, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Error(err)
					os.Exit(2)
				}

				fmt.Println(msg.GetSubject())
			}
		}

		return nil
	})
}

func GetSubject(c *cli.Context) {
	arg := c.Args().First()

	prepareReq(func(client londopb.CertServiceClient) error {
		if arg == "" {
			return argErr
		}

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
	})
}

func prepareReq(f func(londopb.CertServiceClient) error) error {
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
	return f(client)
}

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
