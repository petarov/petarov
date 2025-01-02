package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"time"

	"github.com/google/go-github/github"
	helper "github.com/petarov/petarov/internal"
	"golang.org/x/oauth2"
)

const (
	ThresholdMaxActivityFetch = 50
	ThresholdActivityDays     = 182 * 24 * time.Hour // 6 months
	ThresholdMaxRecentRepos   = 4
	ThresholdReposDays        = 14 * 24 * time.Hour // 14 days
	Username                  = "petarov"
)

var (
	Orgs           = [...]string{"kenamick", "vexelon-dot-net"}
	Excluded       = [...]string{"petarov"}
	ForkExceptions = [...]string{"psiral"}
)

type Entry struct {
	id        int64
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

	index := helper.NewAnySet[int64]()

	pulls, err := fetchLatestPullRequests(ctx, client, index)
	if err != nil {
		log.Fatalf("pull requests: %v\n", err)
	}
	printEntries(pulls)

	issues, err := fetchLatestIssues(ctx, client, index)
	if err != nil {
		log.Fatalf("issues: %v\n", err)
	}
	printEntries(issues)

	comments, err := fetchLatestComments(ctx, client, index)
	if err != nil {
		log.Fatalf("comments: %v\n", err)
	}
	printEntries(comments)

	// Recent repos
	repos, err := getRepositories(ctx, client)
	if err != nil {
		log.Fatalf("repos: %v\n", err)
	}

	if err := writeReadme(repos, pulls, issues, comments); err != nil {
		log.Fatalf("readme.md: %v\n", err)
	}
}

func printEntries(entries []Entry) {
	for _, entry := range entries {
		fmt.Printf("- %s\t\tUpdated: %s\tURL: %s\n", entry.title, entry.updatedAt.Local().Format(time.RFC822), entry.link)
	}
	fmt.Println()
}

func fetchEntries(ctx context.Context, client *github.Client, query string, index *helper.AnySet[int64], traverseComments bool) (entries []Entry, err error) {
	opts := &github.SearchOptions{Sort: "updated", Order: "desc", ListOptions: github.ListOptions{PerPage: ThresholdMaxActivityFetch}}

	then := time.Now().Add(-ThresholdActivityDays)
	query = fmt.Sprintf("%s updated:>%s", query, then.Format("2006-01-02"))

	searchResult, _, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("error fetching entries: %v", err)
	}

	entries = make([]Entry, 0, len(searchResult.Issues))

	for _, issue := range searchResult.Issues {
		addIssue := false
		entryId := issue.GetID()

		if !index.Contains(entryId) {
			if traverseComments {
				// Check when the commenter has commented last in order to extract the correct updatedAt date-time
				if issue.GetComments() > 0 {
					owner, repo, err := helper.ExtractGitHubOwnerAndRepo(issue.GetRepositoryURL())
					if err != nil {
						return nil, err
					}

					comments, _, err := client.Issues.ListComments(ctx, owner, repo, issue.GetNumber(), &github.IssueListCommentsOptions{
						// Sorting and direction do not work when fetching comments for a certain issue
						// See: https://docs.github.com/en/rest/issues/comments?apiVersion=2022-11-28#list-issue-comments
						// Sort:      "created",
						// Direction: "desc",
						Since: then,
						// Fetch no more than 25 comments back. I think more makes little to no sense at this point.
						ListOptions: github.ListOptions{PerPage: 25},
					})
					if err != nil {
						return nil, fmt.Errorf("error fetching '%s' comments list: %v", issue.GetTitle(), err)
					}
					// fmt.Printf("owner,repo = %s %s", owner, repo) // debug
					// fmt.Printf("\t\t\tFound %d comments: %s\n", len(comments), issue.GetTitle()) // debug

					for i := range comments {
						comment := comments[len(comments)-1-i]
						if comment.User.GetLogin() == Username {
							entry := Entry{
								id:        entryId,
								title:     issue.GetTitle(),
								link:      comment.GetHTMLURL(),
								createdAt: comment.GetCreatedAt(),
								updatedAt: comment.GetUpdatedAt(),
							}
							entries = append(entries, entry)
							index.Add(entryId)
							break
						}
					}
				} else {
					// No comments found, so just add the issue
					addIssue = true
					fmt.Printf("!!! No comments found: %s\n", issue.GetTitle())
				}
			} else {
				// Just add the issue or PR without analyzing comments
				addIssue = true
			}
		}

		if addIssue {
			var entry = Entry{
				id:        entryId,
				title:     issue.GetTitle(),
				link:      issue.GetHTMLURL(),
				createdAt: issue.GetCreatedAt(),
				updatedAt: issue.GetUpdatedAt(),
			}
			entries = append(entries, entry)
			index.Add(entryId)
		}
	}

	fmt.Printf("\n*** Query: %s\tResults: %d\n", query, len(entries))

	return entries, nil
}

func fetchLatestIssues(ctx context.Context, client *github.Client, index *helper.AnySet[int64]) (entries []Entry, err error) {
	return fetchEntries(ctx, client, "author:@me type:issue", index, false)
}

func fetchLatestPullRequests(ctx context.Context, client *github.Client, index *helper.AnySet[int64]) (entries []Entry, err error) {
	return fetchEntries(ctx, client, "author:@me type:pr", index, false)
}

func fetchLatestComments(ctx context.Context, client *github.Client, index *helper.AnySet[int64]) (entries []Entry, err error) {
	issues, err := fetchEntries(ctx, client, "commenter:@me is:issue", index, true)
	if err != nil {
		return nil, err
	}

	pulls, err := fetchEntries(ctx, client, "commenter:@me is:pr", index, true)
	if err != nil {
		return nil, err
	}

	all := append(issues, pulls...)

	sort.Slice(all, func(i, j int) bool {
		return all[i].updatedAt.After(all[j].updatedAt)
	})

	return all, nil
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
	then := time.Now().Add(-ThresholdReposDays)
	recent = make([]Entry, 0, ThresholdMaxRecentRepos)

	// user repos
	repos, _, err := client.Repositories.List(ctx, Username, &github.RepositoryListOptions{
		Visibility: "public",
		// Affiliation: "owner,organization_member",
		// Type:        "owner,public",
		Sort:      "pushed",
		Direction: "desc",
		ListOptions: github.ListOptions{
			Page:    0,
			PerPage: 100,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching user repositories: %v", err)
	}

	for _, repo := range repos {
		excluded := isExcluded(repo.GetName()) || repo.GetPushedAt().Before(then)

		if !excluded {
			entry := Entry{
				id:        repo.GetID(),
				title:     repo.GetName(),
				link:      repo.GetHTMLURL(),
				createdAt: repo.GetCreatedAt().Time,
				updatedAt: repo.GetPushedAt().Time,
			}

			// recent commits?
			if len(recent) < ThresholdMaxRecentRepos {
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
		Type:        "public",
	}

	for _, org := range Orgs {
		opt.Page = 0

		for {
			repos, resp, err := client.Repositories.ListByOrg(ctx, org, opt)
			if err != nil {
				return nil, fmt.Errorf("error fetching orga repositories: %v", err)
			}

			for _, repo := range repos {
				excluded := isExcluded(repo.GetName()) || repo.GetPushedAt().Before(then)

				if !excluded {
					entry := Entry{
						id:        repo.GetID(),
						title:     repo.GetName(),
						link:      repo.GetHTMLURL(),
						createdAt: repo.GetCreatedAt().Time,
						updatedAt: repo.GetPushedAt().Time,
					}

					// recent commits?
					if len(recent) < ThresholdMaxRecentRepos {
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

func writeReadme(repos []Entry, pulls []Entry, issues []Entry, comments []Entry) error {
	out, err := os.Create("README.md")
	if err != nil {
		return err
	}

	defer out.Close()

	if len(repos) > 0 {
		out.WriteString(fmt.Sprintf("**recent activity** <sub>past %d days</sub>\n\n", int(ThresholdReposDays.Hours()/24)))

		for _, repo := range repos {
			out.WriteString(fmt.Sprintf("  - **[%s](%s)** - %s\n",
				repo.title, repo.link, helper.GetTimeAgo(repo.updatedAt.Local())))
		}
	}

	if len(pulls) > 0 || len(issues) > 0 || len(comments) > 0 {
		out.WriteString(fmt.Sprintf("\n**pull requests, issues, comments** <sub>past %d months</sub>\n\n", int(math.Ceil(ThresholdActivityDays.Hours()/24/30.44))))

		// Merge everything and sort by date
		all := append(append(pulls, issues...), comments...)

		sort.Slice(all, func(i, j int) bool {
			return all[i].updatedAt.After(all[j].updatedAt)
		})

		for _, entry := range all {
			out.WriteString(fmt.Sprintf("  - **[%s](%s)** - %s\n",
				entry.title, entry.link, helper.GetTimeAgo(entry.updatedAt)))
		}
	}

	out.WriteString("\n<sub>updated: ")
	out.WriteString(time.Now().UTC().Format("2006-01-02 15:04"))
	out.WriteString(" (UTC) | :envelope: gh(@]vexelon.net</sub>")

	return nil
}
