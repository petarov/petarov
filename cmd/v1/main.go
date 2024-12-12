package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/google/go-github/github"
	helper "github.com/petarov/petarov/internal"
	"golang.org/x/oauth2"
)

var (
	MIN_STARS_AND_FORKS = 7
	MAX_RECENT          = 2
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

func getRepositories(token string) (all []*github.Repository, recent []*github.Repository, err error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	all = make([]*github.Repository, 0)
	recent = make([]*github.Repository, 0, MAX_RECENT)

	// user repos
	repos, _, err := client.Repositories.List(ctx, USERNAME, &github.RepositoryListOptions{
		Visibility: "public",
		// Affiliation: "owner,organization_member",
		// Type:        "owner,public",
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching user repositories: %v", err)
	}

	for _, repo := range repos {
		// fmt.Printf("Repo: %s - Stars: %d  Forks: %d \n",
		// 	repo.GetName(), repo.GetStargazersCount(), repo.GetForksCount())

		excluded := isExcluded(repo.GetName())
		forked := repo.GetFork() && !isForkException(repo.GetName())
		if !excluded && !forked && !repo.GetArchived() &&
			repo.GetStargazersCount()+repo.GetForksCount() >= MIN_STARS_AND_FORKS {
			all = append(all, repo)
		}

		if !excluded {
			// recent commits?
			if len(recent) < MAX_RECENT {
				recent = append(recent, repo)
			} else {
				found := -1
				for i, r := range recent {
					if repo.GetPushedAt().After(r.GetPushedAt().Time) &&
						(found == -1 || recent[found].GetPushedAt().After(r.GetPushedAt().Time)) {
						found = i
					}
				}
				if found != -1 {
					recent[found] = repo
				}
			}
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
				return nil, nil, fmt.Errorf("error fetching orga repositories: %v", err)
			}

			for _, repo := range repos {
				excluded := isExcluded(repo.GetName())
				forked := repo.GetFork() && !isForkException(repo.GetName())
				if !excluded && !forked && !repo.GetArchived() &&
					repo.GetStargazersCount()+repo.GetForksCount() >= MIN_STARS_AND_FORKS {
					all = append(all, repo)
				}

				if !excluded {
					// recent commits?
					if len(recent) < MAX_RECENT {
						recent = append(recent, repo)
					} else {
						found := -1
						for i, r := range recent {
							if repo.GetPushedAt().After(r.GetPushedAt().Time) &&
								(found == -1 || recent[found].GetPushedAt().After(r.GetPushedAt().Time)) {
								found = i
							}
						}
						if found != -1 {
							recent[found] = repo
						}
					}
				}
			}

			if resp.NextPage == 0 {
				break
			}

			opt.Page = resp.NextPage
		}
	}

	return all, recent, nil
}

func writeReadme(all []*github.Repository, recent []*github.Repository) error {
	out, err := os.Create("README.md")
	if err != nil {
		return err
	}

	defer out.Close()

	count := 0

	out.WriteString("**Last worked on**\n\n")

	for _, repo := range recent {
		out.WriteString(fmt.Sprintf("  - **[%s](%s)** - %s\n",
			repo.GetName(), repo.GetHTMLURL(), helper.GetTimeAgo(repo.GetPushedAt().Local())))
	}

	out.WriteString("\n**Top 5**\n\n")

	out.WriteString("| :star:+:fork_and_knife: | repo | about | \n")
	out.WriteString("| ----------------------- | ---- | ----- |\n")

	count = 0

	for _, repo := range all {
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

	out.WriteString("\n**Top 5 Gamedev**\n\n")

	out.WriteString("| :star:+:fork_and_knife: | repo | about | \n")
	out.WriteString("| ----------------------- | ---- | ----- |\n")

	count = 0

	for _, repo := range all {
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

func printRepos(repos []*github.Repository) {
	for _, repo := range repos {
		fmt.Printf("Repo: %s/%s\t\tStars: %d  Forks: %d  Lang: %s\n",
			repo.GetOwner().GetLogin(),
			repo.GetName(), repo.GetStargazersCount(), repo.GetForksCount(), repo.GetLanguage())
	}
}

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	all, recent, err := getRepositories(token)
	if err != nil {
		log.Fatalf("error fetching repositories: %v", err)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].GetStargazersCount()+all[i].GetForksCount() >
			all[j].GetStargazersCount()+all[j].GetForksCount()
	})

	sort.Slice(recent, func(i, j int) bool {
		x := recent[i].GetPushedAt()
		y := recent[j].GetPushedAt()
		return x.After(y.Time)
	})

	fmt.Println("-----------")
	printRepos(all)
	fmt.Println("-----------")
	printRepos(recent)

	writeReadme(all, recent)
	if err != nil {
		log.Fatalf("error writing README: %v", err)
	}
}
