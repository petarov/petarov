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
	MIN_STARS_AND_FORKS = 7
	USERNAME            = "petarov"
	ORGS                = [...]string{"kenamick", "vexelon-dot-net"}
	EXCLUDED            = [...]string{"petarov"}
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

func isExcluded(repoName string) bool {
	for _, el := range EXCLUDED {
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
		// fmt.Printf("Repo: %s - Stars: %d  Forks: %d \n",
		// 	repo.GetName(), repo.GetStargazersCount(), repo.GetForksCount())

		excluded := isExcluded(repo.GetName())
		forked := repo.GetFork() && !isForkException(repo.GetName())
		if !excluded && !forked && !repo.GetArchived() &&
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
				excluded := isExcluded(repo.GetName())
				forked := repo.GetFork() && !isForkException(repo.GetName())
				if !excluded && !forked && !repo.GetArchived() &&
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

	count := 0

	out.WriteString("**Last worked on**\n\n")

	timeSorted := make([]*github.Repository, len(repos))
	copy(timeSorted, repos)

	sort.Slice(timeSorted, func(i, j int) bool {
		x := timeSorted[i].GetPushedAt()
		y := timeSorted[j].GetPushedAt()
		return x.After(y.Time)
	})

	for _, repo := range timeSorted {
		out.WriteString(fmt.Sprintf("  - **[%s](%s)** - %s\n",
			repo.GetName(), repo.GetHTMLURL(), getTimeAgo(repo.GetPushedAt().Local())))

		count += 1
		if count == 2 {
			break
		}
	}

	out.WriteString("\n**Top 5**\n\n")

	out.WriteString("| :star:starforked | repo | about | \n")
	out.WriteString("| ---------------- | ---- | ----- |\n")

	count = 0

	for _, repo := range repos {
		if repo.GetOwner().GetLogin() != "kenamick" {
			out.WriteString(fmt.Sprintf("**%d** | **[%s](%s)** | %s\n",
				repo.GetStargazersCount()+repo.GetForksCount(),
				repo.GetName(), repo.GetHTMLURL(), repo.GetDescription()))

			count += 1
			if count == 5 {
				break
			}
		}
	}

	out.WriteString("\n**Top 5 - Games**\n\n")

	out.WriteString("| :star:starforked | repo | about | \n")
	out.WriteString("| ---------------- | ---- | ----- |\n")

	count = 0

	for _, repo := range repos {
		if repo.GetOwner().GetLogin() == "kenamick" {
			out.WriteString(fmt.Sprintf("**%d** | **[%s](%s)** | %s\n",
				repo.GetStargazersCount()+repo.GetForksCount(),
				repo.GetName(), repo.GetHTMLURL(), repo.GetDescription()))

			count += 1
			if count == 5 {
				break
			}
		}
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
			repos[j].GetStargazersCount()+repos[j].GetForksCount()
	})

	for _, repo := range repos {
		if repo.GetStargazersCount()+repo.GetForksCount() >= MIN_STARS_AND_FORKS {
			fmt.Printf("Repo: (%s) %s - %s\nStars: %d  Forks: %d  Lang: %s\n\n",
				repo.GetOwner().GetLogin(),
				repo.GetName(), repo.GetDescription(), repo.GetStargazersCount(), repo.GetForksCount(), repo.GetLanguage())
		}
	}

	writeReadme(repos)
	if err != nil {
		log.Fatalf("error writing README: %v", err)
	}
}
