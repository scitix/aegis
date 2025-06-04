package auth

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// https://github.com/google/go-containerregistry/blob/main/cmd/crane/cmd/auth.go
func NewCommand(argv ...string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Log in or access credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(NewCmdAuthLogin(argv...))

	return cmd
}

type credentials struct {
	Username string
	Secret   string
}

func toCreds(config *authn.AuthConfig) credentials {
	creds := credentials{
		Username: config.Username,
		Secret:   config.Password,
	}

	if config.IdentityToken != "" {
		creds.Username = "<token>"
		creds.Secret = config.IdentityToken
	}
	return creds
}

func NewCmdAuthLogin(argv ...string) *cobra.Command {
	var opts loginOptions

	if len(argv) == 0 {
		argv = []string{os.Args[0]}
	}

	eg := fmt.Sprintf(` $ Log in to reg.example.com
%s login reg.example.com -u hunter -p hunter`, strings.Join(argv, " "))

	cmd := &cobra.Command{
		Use:     "login [OPTION] [SERVER]",
		Short:   "Log in to a registry",
		Example: eg,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := name.NewRegistry(args[0])
			if err != nil {
				return err
			}

			opts.serverAddress = reg.Name()
			return login(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.user, "username", "u", "", "Username")
	flags.StringVarP(&opts.password, "password", "p", "", "Password")
	flags.BoolVarP(&opts.passwordStdin, "password-stdin", "", false, "Take the password from stdin")
	return cmd
}

type loginOptions struct {
	serverAddress string
	user          string
	password      string
	passwordStdin bool
}

func login(opts loginOptions) error {
	if opts.passwordStdin {
		contents, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		opts.password = strings.TrimSuffix(string(contents), "\n")
		opts.password = strings.TrimSuffix(opts.password, "\r")
	}

	if opts.user == "" && opts.password == "" {
		return errors.New("username and password required")
	}

	cf, err := config.Load(os.Getenv("DOCKER_CONFIG"))
	if err != nil {
		return err
	}
	creds := cf.GetCredentialsStore(opts.serverAddress)
	if opts.serverAddress == name.DefaultRegistry {
		opts.serverAddress = authn.DefaultAuthKey
	}
	if err := creds.Store(types.AuthConfig{
		ServerAddress: opts.serverAddress,
		Username:      opts.user,
		Password:      opts.password,
	}); err != nil {
		return err
	}

	if err := cf.Save(); err != nil {
		return err
	}
	klog.Infof("logged to via %s", cf.Filename)
	return nil
}
