package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var (
	MIN_STARS = 7
	USERNAME  = "petarov"
	ORGS      = [...]string{"kenamick", "vexelon-dot-net"}
)

func getRepositories(token string) (result []*github.Repository, err error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	result = make([]*github.Repository, 0)

	repos, _, err := client.Repositories.List(ctx, USERNAME, &github.RepositoryListOptions{
		Visibility: "public",
		// Affiliation: "owner,organization_member",
		// Type:        "owner,public",
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching user repositories: %v", err)
	}

	for _, repo := range repos {
		if *repo.StargazersCount >= MIN_STARS && !*repo.Fork {
			result = append(result, repo)
		}
	}

	// -----
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 20},
	}

	for _, org := range ORGS {
		opt.Page = 0

		for {
			repos, resp, err := client.Repositories.ListByOrg(ctx, org, opt)
			if err != nil {
				return nil, fmt.Errorf("error fetching orga repositories: %v", err)
			}

			for _, repo := range repos {
				if *repo.StargazersCount >= MIN_STARS && !*repo.Fork {
					result = append(result, repo)
				}
			}

			if resp.NextPage == 0 {
				break
			}

			opt.Page = resp.NextPage
		}
	}

	return result, nil
}

func writeReadme(repos []*github.Repository) error {
	out, err := os.Create("README.md")
	if err != nil {
		return err
	}

	defer out.Close()

	for _, repo := range repos {
		forks := ""
		if *repo.ForksCount > 0 {
			forks = fmt.Sprintf("**%d**<sup>:eyes:</sup> ", repo.GetForksCount())
		}
		// lang := ""
		// if len(repo.GetLanguage()) > 0 {
		// 	lang = fmt.Sprintf("<sup>%s</sup> | ", repo.GetLanguage())
		// }

		out.WriteString(fmt.Sprintf("**%d**<sup>:star:</sup> **[%s](%s)** %s| %s\n\n", repo.GetStargazersCount(), repo.GetName(), repo.GetHTMLURL(), forks, *repo.Description))
	}

	out.WriteString("<sub>:envelope: gh(@]vexelon.net</sub>")

	return nil
}

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	repos, err := getRepositories(token)
	if err != nil {
		log.Fatalf("error fetching repositories: %v", err)
	}

	sort.Slice(repos, func(i, j int) bool { return *repos[i].StargazersCount > *repos[j].StargazersCount })

	for _, repo := range repos {
		if *repo.StargazersCount >= MIN_STARS {
			fmt.Printf("Repo: %s - %s\nStars: %d  Forks: %d  Lang: %s\n\n", *repo.Name, *repo.Description, *repo.StargazersCount, *repo.ForksCount,
				repo.GetLanguage())
		}
	}

	writeReadme(repos)
	if err != nil {
		log.Fatalf("error writing README: %v", err)
	}
}
