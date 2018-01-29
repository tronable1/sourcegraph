package repos

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	log15 "gopkg.in/inconshreveable/log15.v2"
	"sourcegraph.com/sourcegraph/sourcegraph/cmd/repo-updater/internal/externalservice/github"
	"sourcegraph.com/sourcegraph/sourcegraph/pkg/api"
	"sourcegraph.com/sourcegraph/sourcegraph/pkg/conf"
	"sourcegraph.com/sourcegraph/sourcegraph/schema"
)

// GitHubServiceType is the (api.ExternalRepoSpec).ServiceType value for GitHub repositories. The ServiceID value
// is the base URL to the GitHub instance (https://github.com or the GitHub Enterprise URL).
const GitHubServiceType = "github"

// GitHubExternalRepoSpec returns an api.ExternalRepoSpec that refers to the specified GitHub repository.
func GitHubExternalRepoSpec(repo *github.Repository, baseURL url.URL) *api.ExternalRepoSpec {
	return &api.ExternalRepoSpec{
		ID:          repo.ID,
		ServiceType: GitHubServiceType,
		ServiceID:   NormalizeGitHubBaseURL(&baseURL).String(),
	}
}

// NormalizeGitHubBaseURL modifies the input and returns a normalized form of the GitHub base URL
// with insignificant differences (such as in presence of a trailing slash, or hostname case)
// eliminated. Its return value should be used for the (ExternalRepoSpec).ServiceID field (and
// passed to GitHubExternalRepoSpec) instead of a non-normalized base URL.
func NormalizeGitHubBaseURL(baseURL *url.URL) *url.URL {
	baseURL.Host = strings.ToLower(baseURL.Host)
	if !strings.HasSuffix(baseURL.Path, "/") {
		baseURL.Path += "/"
	}
	return baseURL
}

// RunGitHubRepositorySyncWorker runs the worker that syncs repositories from the configured GitHub and GitHub
// Enterprise instances to Sourcegraph.
func RunGitHubRepositorySyncWorker(ctx context.Context) error {
	var conns []*githubConnection
	for _, c := range conf.Get().Github {
		conn, err := newGitHubConnection(c)
		if err != nil {
			return fmt.Errorf("error processing GitHub config %s: %s", c.Url, err)
		}
		conns = append(conns, conn)
	}

	if len(conns) == 0 {
		return nil
	}
	for _, c := range conns {
		go func(c *githubConnection) {
			for {
				if rateLimitRemaining, rateLimitReset, ok := c.client.RateLimit(); ok && rateLimitRemaining < 200 {
					wait := rateLimitReset + 10*time.Second
					log15.Warn("GitHub API rate limit is almost exhausted. Waiting until rate limit is reset.", "wait", rateLimitReset, "rateLimitRemaining", rateLimitRemaining)
					time.Sleep(wait)
				}
				updateGitHubRepositories(ctx, c)
				time.Sleep(updateInterval)
			}
		}(c)
	}
	select {}
}

// updateGitHubRepositories ensures that all provided repositories have been added and updated on Sourcegraph.
func updateGitHubRepositories(ctx context.Context, conn *githubConnection) {
	repos := conn.listAllRepositories(ctx)

	githubRepositoryToRepoPath := func(repositoryPathPattern string, repo *github.Repository) api.RepoURI {
		if repositoryPathPattern == "" {
			repositoryPathPattern = "{host}/{nameWithOwner}"
		}
		return api.RepoURI(strings.NewReplacer(
			"{host}", conn.originalHostname,
			"{nameWithOwner}", repo.NameWithOwner,
		).Replace(repositoryPathPattern))
	}

	repoChan := make(chan api.RepoCreateOrUpdateRequest)
	go createEnableUpdateRepos(ctx, nil, repoChan)
	for repo := range repos {
		// log15.Debug("github sync: create/enable/update repo", "repo", repo.NameWithOwner)
		repoChan <- api.RepoCreateOrUpdateRequest{
			RepoURI:      githubRepositoryToRepoPath(conn.config.RepositoryPathPattern, repo),
			ExternalRepo: GitHubExternalRepoSpec(repo, *conn.baseURL),
			Description:  repo.Description,
			Fork:         repo.IsFork,
			Enabled:      conn.config.InitialRepositoryEnablement,
		}
	}
	close(repoChan)
}

func newGitHubConnection(config schema.GitHubConnection) (*githubConnection, error) {
	baseURL, err := url.Parse(config.Url)
	if err != nil {
		return nil, err
	}
	baseURL = NormalizeGitHubBaseURL(baseURL)
	originalHostname := baseURL.Hostname()

	// GitHub.com's API is hosted on api.github.com.
	apiURL := *baseURL
	if hostname := strings.ToLower(apiURL.Hostname()); hostname == "github.com" || hostname == "www.github.com" {
		// GitHub.com
		apiURL = url.URL{Scheme: "https", Host: "api.github.com", Path: "/"}
	} else {
		// GitHub Enterprise
		if apiURL.Path == "" || apiURL.Path == "/" {
			apiURL = *apiURL.ResolveReference(&url.URL{Path: "/api"})
		}
	}

	var transport http.RoundTripper
	if config.Certificate != "" {
		var err error
		transport, err = transportWithCertTrusted(config.Certificate)
		if err != nil {
			return nil, err
		}
	}

	return &githubConnection{
		config:           config,
		baseURL:          baseURL,
		client:           github.NewClient(&apiURL, config.Token, transport),
		originalHostname: originalHostname,
	}, nil
}

type githubConnection struct {
	config  schema.GitHubConnection
	baseURL *url.URL
	client  *github.Client

	// originalHostname is the hostname of config.Url (differs from client APIURL, whose host is api.github.com
	// for an originalHostname of github.com).
	originalHostname string
}

func (c *githubConnection) listAllRepositories(ctx context.Context) <-chan *github.Repository {
	const first = 100 // max GitHub API "first" parameter
	ch := make(chan *github.Repository, first)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if len(c.config.RepositoryQuery) == 0 {
			// Users need to specify ["none"] to disable affiliated default.
			c.config.RepositoryQuery = []string{"affiliated"}
		}
		for _, repositoryQuery := range c.config.RepositoryQuery {
			switch repositoryQuery {
			case "affiliated":
				var endCursor *string // GraphQL pagination cursor
				for {
					var repos []*github.Repository
					var rateLimitCost int
					var err error
					repos, endCursor, rateLimitCost, err = c.client.ListViewerRepositories(ctx, first, endCursor)
					if err != nil {
						log15.Error("Error listing viewer's affiliated GitHub repositories", "endCursor", endCursor, "error", err)
						break
					}
					rateLimitRemaining, rateLimitReset, _ := c.client.RateLimit()
					log15.Debug("github sync: ListViewerRepositories", "repos", len(repos), "rateLimitCost", rateLimitCost, "rateLimitRemaining", rateLimitRemaining, "rateLimitReset", rateLimitReset)
					for _, r := range repos {
						// log15.Debug("github sync: ListViewerRepositories: repo", "repo", r.NameWithOwner)
						ch <- r
					}
					if endCursor == nil {
						break
					}
					time.Sleep(c.client.RecommendedRateLimitWaitForBackgroundOp(rateLimitCost))
				}

			case "none":
				// nothing to do

			default:
				log15.Error("Skipping unrecognized GitHub configuration repositoryQuery", "repositoryQuery", repositoryQuery)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, nameWithOwner := range c.config.Repos {
			owner, name, err := github.SplitRepositoryNameWithOwner(nameWithOwner)
			if err != nil {
				log15.Error("Invalid GitHub repository", "nameWithOwner", nameWithOwner)
				continue
			}
			repo, err := c.client.GetRepository(ctx, owner, name)
			if err != nil {
				log15.Error("Error getting GitHub repository", "nameWithOwner", nameWithOwner, "error", err)
				continue
			}
			log15.Debug("github sync: GetRepository", "repo", repo.NameWithOwner)
			ch <- repo
			time.Sleep(c.client.RecommendedRateLimitWaitForBackgroundOp(1)) // 0-duration sleep unless nearing rate limit exhaustion
		}
	}()

	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch
}
