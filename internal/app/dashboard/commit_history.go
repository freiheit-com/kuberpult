// Package dashboard contains the dashboard app logic.
package dashboard

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/freiheit-com/kuberpult/internal/app/dashboard/model"
	"github.com/freiheit-com/kuberpult/internal/pkg/errors"
	"github.com/freiheit-com/kuberpult/internal/pkg/k8s"
)

// CommitHistory represents the commit history of a repository.
type CommitHistory struct {
	Commits []model.Commit
}

// GetCommitHistory returns the commit history of a repository.
func GetCommitHistory(ctx context.Context, repo model.Repository) (*CommitHistory, error) {
	commits, err := getCommits(ctx, repo)
	if err != nil {
		return nil, err
	}
	
	// Remove the limit of 100 events
	// commits = commits[:100]
	
	// Sort the commits in descending order by timestamp
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Timestamp.After(commits[j].Timestamp)
	})
	
	return &CommitHistory{Commits: commits}, nil
}

func getCommits(ctx context.Context, repo model.Repository) ([]model.Commit, error) {
	// Fetch commits from the repository
	commits, err := k8s.GetCommits(ctx, repo)
	if err != nil {
		return nil, err
	}
	
	// Convert the commits to the desired format
	var formattedCommits []model.Commit
	for _, commit := range commits {
		formattedCommits = append(formattedCommits, model.Commit{
			ID:        commit.ID,
			Message:   commit.Message,
			Author:    commit.Author,
			Timestamp: commit.Timestamp,
		})
	}
	
	return formattedCommits, nil
}
