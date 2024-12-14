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

var (
	THRESHOLD_FETCH        = 25
	THRESHOLD_DAYS         = 180 * 24 * time.Hour
	THRESHOLD_ENTRIES      = 10
	THRESHOLD_RECENT_REPOS = 2
)

var (
	USERNAME        = "petarov"
	ORGS            = [...]string{"kenamick", "vexelon-dot-net"}
	EXCLUDED        = [...]string{"petarov"}
	FORK_EXCEPTIONS = [...]string{"psiral"}
)

type Entry struct {
	title     string
	link      string
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

	comments, err := fetchLatestComments(ctx, client)
	if err != nil {
		log.Fatalf("comments: %v\n", err)
	}
	printEntries("Comments", comments)

	// merge and sort issues and comments
	issuesAndComments := append(issues, comments...)

	sort.Slice(issuesAndComments, func(i, j int) bool {
		return issuesAndComments[i].updatedAt.After(issuesAndComments[j].updatedAt)
	})

	// recent repos
	repos, err := getRepositories(ctx, client)
	if err != nil {
		log.Fatalf("repos: %v\n", err)
	}

	if err := writeReadme(repos, pulls, issuesAndComments); err != nil {
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
	opts := &github.SearchOptions{Sort: "updated", Order: "desc", ListOptions: github.ListOptions{PerPage: THRESHOLD_FETCH}}

	searchResult, _, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("error fetching entries: %v", err)
	}

	entries = make([]Entry, 0, THRESHOLD_ENTRIES)
	then := time.Now().Add(-THRESHOLD_DAYS)

	for i, issue := range searchResult.Issues {
		entry := Entry{title: issue.GetTitle(), link: issue.GetHTMLURL(), updatedAt: *issue.UpdatedAt}
		if issue.GetUpdatedAt().After(then) {
			entries = append(entries, entry)
		}

		if i > THRESHOLD_ENTRIES {
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
	for _, el := range EXCLUDED {
		if el == repoName {
			return true
		}
	}
	return false
}

func getRepositories(ctx context.Context, client *github.Client) (recent []Entry, err error) {
	recent = make([]Entry, 0, THRESHOLD_RECENT_REPOS)

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
		excluded := isExcluded(repo.GetName())

		if !excluded {
			entry := Entry{title: repo.GetName(), link: repo.GetURL(), updatedAt: repo.GetPushedAt().Time}

			// recent commits?
			if len(recent) < THRESHOLD_RECENT_REPOS {
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

	for _, org := range ORGS {
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
					if len(recent) < THRESHOLD_RECENT_REPOS {
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

func writeReadme(recent []Entry, pulls []Entry, issuesAndComments []Entry) error {
	out, err := os.Create("README.md")
	if err != nil {
		return err
	}

	defer out.Close()

	out.WriteString("**last worked on**\n\n")

	for _, repo := range recent {
		out.WriteString(fmt.Sprintf("  - **[%s](%s)** - %s\n",
			repo.title, repo.link, helper.GetTimeAgo(repo.updatedAt.Local())))
	}

	if len(pulls) > 0 {
		out.WriteString("\n**last pull requests**\n\n")

		for _, pr := range pulls {
			out.WriteString(fmt.Sprintf("  - **[%s](%s)** - %s\n",
				pr.title, pr.link, helper.GetTimeAgo(pr.updatedAt)))
		}
	}

	if len(issuesAndComments) > 0 {
		out.WriteString("\n**last issues & comments**\n\n")

		for _, isc := range issuesAndComments {
			out.WriteString(fmt.Sprintf("  - **[%s](%s)** - %s\n",
				isc.title, isc.link, helper.GetTimeAgo(isc.updatedAt)))
		}
	}

	out.WriteString("\n<sub>:envelope: gh(@]vexelon.net</sub>")

	return nil
}
