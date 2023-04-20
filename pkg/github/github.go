// Setup GitHub client
package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/github"
	"github.com/thejokersthief/banshee/pkg/configs"
	"golang.org/x/oauth2"
)

type GithubClient struct {
	client *github.Client
	ctx    context.Context
}

func NewGithubClient(globalConf configs.GlobalConfig, ctx context.Context) (*GithubClient, error) {
	ghClient := &GithubClient{ctx: ctx}

	if globalConf.Github.UseGithubApp {

		configMissing := (globalConf.Github.AppID == 0 ||
			globalConf.Github.AppInstallationID == 0 ||
			globalConf.Github.AppPrivateKeyPath == "")
		if configMissing {
			return nil, fmt.Errorf("Config missing for AppID, InstallationID or PrivateKeyPath")
		}

		itr, err := ghinstallation.NewKeyFromFile(
			http.DefaultTransport,
			globalConf.Github.AppID,
			globalConf.Github.AppInstallationID,
			globalConf.Github.AppPrivateKeyPath,
		)

		if err != nil {
			return nil, err
		}

		// Use installation transport with client.
		ghClient.client = github.NewClient(&http.Client{Transport: itr})
	} else {

		configMissing := (globalConf.Github.Token == "")
		if configMissing {
			return nil, fmt.Errorf("Config missing for GitHub Token")
		}

		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: globalConf.Github.Token},
		)
		tc := oauth2.NewClient(ctx, ts)

		ghClient.client = github.NewClient(tc)
	}

	return ghClient, nil
}
