package cli

import (
	"context"
	"fmt"
	"google.golang.org/grpc/credentials"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/alexyermolaev/londo"
	"github.com/alexyermolaev/londo/jwt"
	"github.com/alexyermolaev/londo/londopb"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
)

const (
	Copyright = "(c) 2019 Alex Yermolaev, MIT License"
)

var (
	authors = cli.Author{
		Name: "Alex Yermolaev",
	}

	argErr = cli.NewExitError("must specify an argument", 1)
	err    error

	token       *Token
	server      *Server
	certPath    *CertPath
	ExpDays     int
	SFile       string
	CAFile      string
	UpdateCerts bool
)

func init() {
	log.SetFormatter(&prefixed.TextFormatter{
		FullTimestamp:    true,
		DisableUppercase: true,
	})

	token = &Token{}
	server = &Server{}
	certPath = &CertPath{}

}

func GetCopyright() string {
	return Copyright
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

	if err := DoRequest(c, func(client londopb.CertServiceClient) error {
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
			req := &londopb.ForTargetRequest{Update: UpdateCerts}

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
					log.Fatal(err)
				}

				log.Infof("saving %s certificate", msg.GetSubject().GetSubject())
				if err = SaveCert(msg.GetSubject()); err != nil {
					log.Fatal(err)
				}
			}
		}

		return nil
	}); err != nil {
		log.Fatal(err)
	}
}

func AddSubject(c *cli.Context) error {
	if !c.Args().Present() {
		return argErr
	}

	DoRequest(c, func(client londopb.CertServiceClient) error {
		req := &londopb.AddNewSubjectRequest{
			Subject: &londopb.NewSubject{
				Subject:  c.Args().First(),
				Port:     int32(c.Int("port")),
				AltNames: c.StringSlice("alt"),
				Targets:  c.StringSlice("target"),
			},
		}

		res, err := client.AddNewSubject(context.Background(), req)
		if err != nil {
			log.Fatal(err)
		}

		log.Info(res.GetSubject())

		return nil
	})

	return nil
}

func DeleteSubject(c *cli.Context) error {
	if !c.Args().Present() {
		return argErr
	}

	DoRequest(c, func(client londopb.CertServiceClient) error {
		req := &londopb.DeleteSubjectRequest{
			Subject: c.Args().First(),
		}

		res, err := client.DeleteSubject(context.Background(), req)
		if err != nil {
			log.Fatal(err)
		}

		log.Info(res.GetSubject())

		return nil
	})

	return nil
}

func RenewSubject(c *cli.Context) {
	if !c.Args().Present() {
		return
	}

	DoRequest(c, func(client londopb.CertServiceClient) error {
		req := &londopb.RenewSubjectRequest{
			Subject: c.Args().First(),
		}

		stream, err := client.RenewSubjects(context.Background(), req)
		if err != nil {
			log.Fatal(err)
		}

		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				break
			}

			if err != nil {
				log.Fatal(err)
			}

			log.Infof("%s was queued to be renewed", msg.GetSubject().GetSubject())
		}

		return nil
	})
}

func GetExpiringSubjects(c *cli.Context) {
	DoRequest(c, func(client londopb.CertServiceClient) error {
		req := &londopb.GetExpiringSubjectsRequest{
			Days: int32(ExpDays),
		}

		stream, err := client.GetExpiringSubject(context.Background(), req)
		if err != nil {
			log.Fatal(err)
		}

		log.Info("results:")
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				break
			}

			if err != nil {
				log.Fatal(err)
			}

			t := time.Unix(msg.GetSubject().ExpDate, 0).String()

			y := ExpiringSubjects{
				Subject:  msg.GetSubject().GetSubject(),
				NotAfter: t,
			}

			s, err := yaml.Marshal(&y)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(string(s))
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
			log.Fatal(err)
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

	creds, err := credentials.NewClientTLSFromFile(CAFile, "")
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("dialing %s", server.String)
	conn, err := grpc.Dial(server.String, grpc.WithTransportCredentials(creds), grpc.WithPerRPCCredentials(auth))
	if err != nil {
		return cli.NewExitError("unable to connect.", 127)
	}

	defer conn.Close()

	client := londopb.NewCertServiceClient(conn)
	return f(client)
}

func IssueToken(c *cli.Context) error {
	jwt.ReadSecret(SFile)

	arg := c.Args().First()

	if arg == "" {
		return argErr
	}

	if err != nil {
		return cli.NewExitError("cannot read config", 1)
	}

	b, err := jwt.IssueJWT(arg)
	if err != nil {
		return cli.NewExitError(err, 1)
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

func NewToken() *Token {
	return token
}

type Server struct {
	String string
}

func NewServer() *Server {
	return server
}

type CertPath struct {
	Public  string
	Private string
}

func NewCertPath() *CertPath {
	return certPath
}

func SaveCert(s *londopb.Subject) error {
	cert := []byte(s.GetCertificate())
	prv := []byte(s.GetPrivateKey())

	subj := s.GetSubject()
	pub := certPath.Public + "/" + subj + ".crt"
	if err := ioutil.WriteFile(pub, cert, 0644); err != nil {
		return err
	}

	key := certPath.Private + "/" + subj + ".key"
	if err := ioutil.WriteFile(key, prv, 0600); err != nil {
		return err
	}

	return nil
}

type ExpiringSubjects struct {
	Subject  string `yaml:"subject"`
	NotAfter string `yaml:"not_after"`
}

func DaemonSetup(name string, usage string, action interface{}) *cli.App {
	app := cli.NewApp()

	app.Name = name
	app.Usage = usage
	app.Version = londo.Version
	app.Copyright = Copyright
	app.Authors = []cli.Author{GetAuthors()}

	app.Flags = londo.DefaultFlags

	app.Action = action

	return app
}
