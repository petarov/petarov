package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/google/go-github/github"
	helper "github.com/petarov/petarov/internal"
	"golang.org/x/oauth2"
)

const (
	ThresholdFetch       = 25
	ThresholdDays        = 180 * 24 * time.Hour
	ThresholdEntries     = 10
	ThresholdRecentRepos = 2
	Username             = "petarov"
)

var (
	Orgs           = [...]string{"kenamick", "vexelon-dot-net"}
	Excluded       = [...]string{"petarov"}
	ForkExceptions = [...]string{"psiral"}
)

type Entry struct {
	title     string
	link      string
	createdAt time.Time
	updatedAt time.Time
}

func main() {
	token := os.Getenv("GITHUB_TOKEN")

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	pulls, err := fetchLatestPullRequests(ctx, client)
	if err != nil {
		log.Fatalf("pull requests: %v\n", err)
	}
	printEntries("PRs", pulls)

	issues, err := fetchLatestIssues(ctx, client)
	if err != nil {
		log.Fatalf("issues: %v\n", err)
	}
	printEntries("Issues", issues)

	// merge and sort issues and pull requests
	pullsAndIssues := append(pulls, issues...)

	sort.Slice(pullsAndIssues, func(i, j int) bool {
		return pullsAndIssues[i].updatedAt.After(pullsAndIssues[j].updatedAt)
	})

	comments, err := fetchLatestComments(ctx, client)
	if err != nil {
		log.Fatalf("comments: %v\n", err)
	}
	printEntries("Comments", comments)

	// recent repos
	repos, err := getRepositories(ctx, client)
	if err != nil {
		log.Fatalf("repos: %v\n", err)
	}

	if err := writeReadme(repos, pullsAndIssues, comments); err != nil {
		log.Fatalf("readme.md: %v\n", err)
	}
}

func printEntries(name string, entries []Entry) {
	fmt.Printf("------  List of %s...\n", name)
	for _, entry := range entries {
		fmt.Printf("- %s\t\tUpdated: %s\tURL: %s\n", entry.title, entry.updatedAt.Local().Format(time.RFC822), entry.link)
	}

	fmt.Println()
}

func fetchEntries(ctx context.Context, client *github.Client, query string) (entries []Entry, err error) {
	opts := &github.SearchOptions{Sort: "updated", Order: "desc", ListOptions: github.ListOptions{PerPage: ThresholdFetch}}

	searchResult, _, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("error fetching entries: %v", err)
	}

	entries = make([]Entry, 0, ThresholdEntries)
	then := time.Now().Add(-ThresholdDays)

	for i, issue := range searchResult.Issues {
		entry := Entry{title: issue.GetTitle(),
			link:      issue.GetHTMLURL(),
			createdAt: issue.GetCreatedAt(),
			updatedAt: issue.GetUpdatedAt(),
		}
		if issue.GetUpdatedAt().After(then) {
			entries = append(entries, entry)
		}

		if i > ThresholdEntries {
			break
		}
	}

	return entries, nil
}

func fetchLatestIssues(ctx context.Context, client *github.Client) (entries []Entry, err error) {
	return fetchEntries(ctx, client, "author:@me type:issue")
}

func fetchLatestPullRequests(ctx context.Context, client *github.Client) (entries []Entry, err error) {
	return fetchEntries(ctx, client, "author:@me type:pr")
}

func fetchLatestComments(ctx context.Context, client *github.Client) (entries []Entry, err error) {
	return fetchEntries(ctx, client, "is:issue commenter:@me")
}

func isExcluded(repoName string) bool {
	for _, el := range Excluded {
		if el == repoName {
			return true
		}
	}
	return false
}

func getRepositories(ctx context.Context, client *github.Client) (recent []Entry, err error) {
	recent = make([]Entry, 0, ThresholdRecentRepos)

	// user repos
	repos, _, err := client.Repositories.List(ctx, Username, &github.RepositoryListOptions{
		Visibility: "public",
		// Affiliation: "owner,organization_member",
		// Type:        "owner,public",
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching user repositories: %v", err)
	}

	for _, repo := range repos {
		excluded := isExcluded(repo.GetName())

		if !excluded {
			entry := Entry{title: repo.GetName(), link: repo.GetURL(), updatedAt: repo.GetPushedAt().Time}

			// recent commits?
			if len(recent) < ThresholdRecentRepos {
				recent = append(recent, entry)
			} else {
				found := -1
				for i, r := range recent {
					if repo.GetPushedAt().After(r.updatedAt) &&
						(found == -1 || recent[found].updatedAt.After(r.updatedAt)) {
						found = i
					}
				}
				if found != -1 {
					recent[found] = entry
				}
			}
		}
	}

	// orgas

	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 20},
	}

	for _, org := range Orgs {
		opt.Page = 0

		for {
			repos, resp, err := client.Repositories.ListByOrg(ctx, org, opt)
			if err != nil {
				return nil, fmt.Errorf("error fetching orga repositories: %v", err)
			}

			for _, repo := range repos {
				excluded := isExcluded(repo.GetName())

				if !excluded {
					entry := Entry{title: repo.GetName(), link: repo.GetURL(), updatedAt: repo.GetPushedAt().Time}

					// recent commits?
					if len(recent) < ThresholdRecentRepos {
						recent = append(recent, entry)
					} else {
						found := -1
						for i, r := range recent {
							if repo.GetPushedAt().After(r.updatedAt) &&
								(found == -1 || recent[found].updatedAt.After(r.updatedAt)) {
								found = i
							}
						}
						if found != -1 {
							recent[found] = entry
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

	return recent, nil
}

func writeReadme(recent []Entry, pullsAndIssues []Entry, comments []Entry) error {
	out, err := os.Create("README.md")
	if err != nil {
		return err
	}

	defer out.Close()

	out.WriteString("**recent activity**\n\n")

	for _, repo := range recent {
		out.WriteString(fmt.Sprintf("  - **[%s](%s)** - %s\n",
			repo.title, repo.link, helper.GetTimeAgo(repo.updatedAt.Local())))
	}

	if len(pullsAndIssues) > 0 {
		out.WriteString("\n**issues & pull requests**\n\n")

		for _, pr := range pullsAndIssues {
			out.WriteString(fmt.Sprintf("  - **[%s](%s)** - %s\n",
				pr.title, pr.link, helper.GetTimeAgo(pr.updatedAt)))
		}
	}

	if len(comments) > 0 {
		out.WriteString("\n**comments**\n\n")

		for _, isc := range comments {
			out.WriteString(fmt.Sprintf("  - **[%s](%s)** - %s\n",
				isc.title, isc.link, helper.GetTimeAgo(isc.updatedAt)))
		}
	}

	out.WriteString("\n<sub>:envelope: gh(@]vexelon.net</sub>")

	return nil
}
