package remediation

import (
	"context"
	"fmt"
	"time"

	gh "github.com/google/go-github/v62/github"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// PRCreator opens a GitHub pull request with a remediation patch
type PRCreator struct {
	token string
}

func NewPRCreator(token string) *PRCreator {
	return &PRCreator{token: token}
}

// PRRequest contains everything needed to open a PR
type PRRequest struct {
	RepoOwner    string
	RepoName     string
	BaseBranch   string
	FilePath     string
	Patch        string
	DriftID      uuid.UUID
	ResourceName string
	Severity     string
}

// PRResult contains PR output
type PRResult struct {
	PRURL    string
	PRNumber int
}

func (p *PRCreator) OpenPR(ctx context.Context, req PRRequest) (*PRResult, error) {

	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: p.token,
	})
	tc := oauth2.NewClient(ctx, ts)
	client := gh.NewClient(tc)

	branchName := fmt.Sprintf("driftguard/fix-%s-%s",
		req.ResourceName,
		time.Now().Format("20060102-150405"),
	)

	// Get base branch SHA
	baseRef, _, err := client.Git.GetRef(ctx,
		req.RepoOwner,
		req.RepoName,
		"refs/heads/"+req.BaseBranch,
	)
	if err != nil {
		return nil, fmt.Errorf("getting base ref: %w", err)
	}

	baseSHA := baseRef.Object.GetSHA()

	// Create branch
	newRef := &gh.Reference{
		Ref: gh.String("refs/heads/" + branchName),
		Object: &gh.GitObject{
			SHA: gh.String(baseSHA),
		},
	}

	_, _, err = client.Git.CreateRef(ctx,
		req.RepoOwner,
		req.RepoName,
		newRef,
	)
	if err != nil {
		return nil, fmt.Errorf("creating branch: %w", err)
	}

	// Get file (if exists)
	fileContent, _, resp, err := client.Repositories.GetContents(
		ctx,
		req.RepoOwner,
		req.RepoName,
		req.FilePath,
		&gh.RepositoryContentGetOptions{Ref: req.BaseBranch},
	)

	var fileSHA string
	if resp != nil && resp.StatusCode == 404 {
		fileSHA = ""
	} else if err != nil {
		return nil, fmt.Errorf("getting file content: %w", err)
	} else {
		fileSHA = fileContent.GetSHA()
	}

	// Commit message
	commitMsg := fmt.Sprintf(
		"fix(%s): remediate %s drift [DriftGuard #%s]",
		req.Severity,
		req.ResourceName,
		req.DriftID.String()[:8],
	)

	opts := &gh.RepositoryContentFileOptions{
		Message: gh.String(commitMsg),
		Content: []byte(req.Patch),
		Branch:  gh.String(branchName),
		Committer: &gh.CommitAuthor{
			Name:  gh.String("DriftGuard Bot"),
			Email: gh.String("bot@driftguard.io"),
		},
	}

	if fileSHA != "" {
		opts.SHA = gh.String(fileSHA)
	}

	_, _, err = client.Repositories.UpdateFile(
		ctx,
		req.RepoOwner,
		req.RepoName,
		req.FilePath,
		opts,
	)
	if err != nil {
		_, _, err = client.Repositories.CreateFile(
			ctx,
			req.RepoOwner,
			req.RepoName,
			req.FilePath,
			opts,
		)
		if err != nil {
			return nil, fmt.Errorf("creating/updating file: %w", err)
		}
	}

	// Create PR
	prTitle := fmt.Sprintf(
		"[DriftGuard] Fix %s drift (%s)",
		req.ResourceName,
		req.Severity,
	)

	prBody := fmt.Sprintf(`## DriftGuard Automated Remediation

**Drift ID:** %s  
**Resource:** %s  
**Severity:** %s  

This PR was automatically generated.

Review carefully before merging.
`,
		req.DriftID.String(),
		req.ResourceName,
		req.Severity,
	)

	pr, _, err := client.PullRequests.Create(ctx,
		req.RepoOwner,
		req.RepoName,
		&gh.NewPullRequest{
			Title: gh.String(prTitle),
			Body:  gh.String(prBody),
			Head:  gh.String(branchName),
			Base:  gh.String(req.BaseBranch),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("creating PR: %w", err)
	}

	return &PRResult{
		PRURL:    pr.GetHTMLURL(),
		PRNumber: pr.GetNumber(),
	}, nil
}
