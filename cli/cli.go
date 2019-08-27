package cli

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	jwt "github.com/alexyermolaev/londo/jwt"
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
	err    error

	token *Token
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	token = &Token{}
}

func NewToken() *Token {
	return token
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

func CallCertService(c *cli.Context) {
	log.Info("reading token")
	token.Read(c)

	log.Info("updating token")
	UpdateToken(c)

	log.Info("downloading certificates")
	GetForTarget(c)
}

func UpdateToken(c *cli.Context) error {
	DoRequest(c, func(client londopb.CertServiceClient) error {
		req := &londopb.GetTokenRequest{}

		res, err := client.GetToken(context.Background(), req)
		if err != nil {
			log.Fatal(err)
		}

		log.Infof("saving token to %s", token.File)
		token.String = res.Token.Token
		if err := token.Save(); err != nil {
			log.Fatalf("unable to save updated token in %s", token.File)
		}

		return nil
	})

	return nil
}

func GetForTarget(c *cli.Context) {
	arg := c.Args().First()
	var targets []string
	targets = append(targets, arg)

	DoRequest(c, func(client londopb.CertServiceClient) error {
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

	DoRequest(c, func(client londopb.CertServiceClient) error {
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

func DoRequest(c *cli.Context, f func(londopb.CertServiceClient) error) error {
	auth := &authCreds{
		token: token.String,
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

func IssueToken(c *cli.Context) error {
	arg := c.Args().First()

	if arg == "" {
		return argErr
	}

	if err != nil {
		return cli.NewExitError("cannot read config", 1)
	}

	b, err := jwt.IssueJWT(arg)
	if err != nil {
		return cli.NewExitError("cannot issue a token", 1)
	}

	token.String = string(b)

	fmt.Printf("sub: %s token: %s\n", arg, string(token.String))
	return nil
}

type Token struct {
	File   string
	String string
}

func (t *Token) Read(c *cli.Context) error {

	b, err := ioutil.ReadFile(t.File)
	if err != nil {
		return err
	}

	t.String = strings.Trim(string(b), "\n")

	return nil
}

func (t *Token) Save() error {
	os.Remove(t.File)
	bt := []byte(t.String)
	return ioutil.WriteFile(t.File, bt, 0400)
}
