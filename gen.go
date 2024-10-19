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
	MIN_STARS_AND_FORKS = 10
	USERNAME            = "petarov"
	ORGS                = [...]string{"kenamick", "vexelon-dot-net"}
	FORK_EXCEPTIONS     = [...]string{"psiral"}
)

func isForkException(repoName string) bool {
	for _, el := range FORK_EXCEPTIONS {
		if el == repoName {
			return true
		}
	}
	return false
}

func getRepositories(token string) (result []*github.Repository, err error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	result = make([]*github.Repository, 0)

	// user repos

	repos, _, err := client.Repositories.List(ctx, USERNAME, &github.RepositoryListOptions{
		Visibility: "public",
		// Affiliation: "owner,organization_member",
		// Type:        "owner,public",
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching user repositories: %v", err)
	}

	for _, repo := range repos {
		forked := repo.GetFork() && !isForkException(repo.GetName())
		if !forked && !repo.GetArchived() &&
			repo.GetStargazersCount()+repo.GetForksCount() >= MIN_STARS_AND_FORKS {
			result = append(result, repo)
		}
	}

	// orgas

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
				forked := repo.GetFork() && !isForkException(repo.GetName())
				if !forked && !repo.GetArchived() &&
					repo.GetStargazersCount()+repo.GetForksCount() >= MIN_STARS_AND_FORKS {
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

	out.WriteString("| :star:starforked | repo | about | \n")
	out.WriteString("| ---------------- | ---- | ----- |\n")

	for _, repo := range repos {

		out.WriteString(fmt.Sprintf("**%d** | **[%s](%s)** | %s\n",
			repo.GetStargazersCount()+repo.GetForksCount(),
			repo.GetName(), repo.GetHTMLURL(), repo.GetDescription()))
	}

	out.WriteString("\n<sub>:envelope: gh(@]vexelon.net</sub>")

	return nil
}

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	repos, err := getRepositories(token)
	if err != nil {
		log.Fatalf("error fetching repositories: %v", err)
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].GetStargazersCount()+repos[i].GetForksCount() >
			repos[j].GetStargazersCount()+repos[i].GetForksCount()
	})

	for _, repo := range repos {
		if repo.GetStargazersCount()+repo.GetForksCount() >= MIN_STARS_AND_FORKS {
			fmt.Printf("Repo: %s - %s\nStars: %d  Forks: %d  Lang: %s\n\n",
				repo.GetName(), repo.GetDescription(), repo.GetStargazersCount(), repo.GetForksCount(), repo.GetLanguage())
		}
	}

	writeReadme(repos)
	if err != nil {
		log.Fatalf("error writing README: %v", err)
	}
}
