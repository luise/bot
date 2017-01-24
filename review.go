package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/google/go-github/github"
)

type review struct {
	State string
	User  github.User
}

func runReview(client *github.Client) {
	prs, _, err := client.PullRequests.List("Netsys", "quilt", nil)
	if err != nil {
		fmt.Println("Failed to list pull requests: ", err)
		return
	}

	for _, pr := range prs {
		processPullRequest(client, pr)
	}
}

var memberIndex = rand.Intn(100)
var committerIndex = rand.Intn(100)

func processPullRequest(client *github.Client, pr *github.PullRequest) {
	members, committers := getTeamMembers(client)

	committerSet := map[string]struct{}{}
	for _, c := range committers {
		committerSet[c] = struct{}{}
	}

	var assignees []string
	for _, user := range pr.Assignees {
		if _, ok := committerSet[*user.Login]; ok {
			// The pull request has already been assigned to a
			// committer, so nothing more for quilt-bot to do.
			return
		}
		assignees = append(assignees, *user.Login)
	}

	var assignee string
	if len(assignees) == 0 {
		assignee = chooseReviewer(members, &memberIndex, pr)
	} else {
		reviews, err := getReviews(client, pr)
		if err != nil {
			fmt.Println("Failed to check for approval: ", err)
			return
		}

		for _, review := range reviews {
			if review.State == "APPROVED" {
				if _, ok := committerSet[*pr.User.Login]; ok {
					// PRs opened by a committer need no further
					// approval.
					assignee = *pr.User.Login
				} else {
					assignee = chooseReviewer(committers,
						&committerIndex, pr)
				}
				break
			}
		}
	}

	if assignee == "" {
		return
	}

	if err := assignPullRequest(client, pr, assignee); err != nil {
		fmt.Printf("Failed to assign %s to PR %d: %s\n", assignee,
			*pr.Number, assignee)
	}
}

func chooseReviewer(options []string, index *int, pr *github.PullRequest) string {
	for i := 0; i < len(options); i++ {
		*index++
		choice := options[*index%len(options)]
		if choice != *pr.User.Login {
			return choice
		}
	}

	return ""
}

func getReviews(client *github.Client, pr *github.PullRequest) ([]review, error) {
	var reviews []review
	err := prRequest(client, pr, "GET", "reviews", nil, &reviews)
	return reviews, err
}

func assignPullRequest(client *github.Client, pr *github.PullRequest,
	login string) error {

	fmt.Printf("Assign Pull Request %d review to %s\n", *pr.Number, login)

	_, _, err := client.Issues.AddAssignees("Netsys", "quilt", *pr.Number,
		[]string{login})
	if err != nil {
		return err
	}

	if *pr.User.Login == login {
		// This happens in the case when we assign a committer to their own
		// review.
		return nil
	}

	post := map[string][]string{"reviewers": []string{login}}
	return prRequest(client, pr, "POST", "requested_reviewers", &post, nil)
}

func prRequest(client *github.Client, pr *github.PullRequest, method,
	action string, post, result interface{}) error {

	url := fmt.Sprintf("/repos/Netsys/quilt/pulls/%d/%s", *pr.Number, action)
	req, err := client.NewRequest(method, url, post)
	if err != nil {
		return err
	}

	// This API isn't ready yet, so we have to disclaim with a magic header.
	req.Header.Set("Accept", "application/vnd.github.black-cat-preview+json")

	_, err = client.Do(req, result)
	return err
}

var cachedMembers, cachedCommitters []string
var memberRateLimit = time.Tick(time.Hour)

func getTeamMembers(client *github.Client) (members, committers []string) {
	select {
	case <-memberRateLimit:
	default:
		if cachedMembers != nil && cachedCommitters != nil {
			return cachedMembers, cachedCommitters
		}
	}

	teams, _, err := client.Organizations.ListTeams("Netsys", nil)
	if err != nil {
		fmt.Println("Failed to list teams: ", err)
		return cachedMembers, cachedCommitters
	}

	var memberID, committerID int
	for _, team := range teams {
		switch *team.Name {
		case "Quilt":
			memberID = *team.ID
		case "Quilt Committers":
			committerID = *team.ID
		}
	}

	newMembers, _, err := client.Organizations.ListTeamMembers(memberID, nil)
	if err != nil {
		fmt.Println("Failed to list team members: ", err)
		return cachedMembers, cachedCommitters
	}

	newCommitters, _, err := client.Organizations.ListTeamMembers(committerID, nil)
	if err != nil {
		fmt.Println("Failed to list committers: ", err)
		return cachedMembers, cachedCommitters
	}

	cachedCommitters = []string{}
	committerSet := map[string]struct{}{}
	for _, c := range newCommitters {
		committer := *c.Login
		cachedCommitters = append(cachedCommitters, committer)
		committerSet[committer] = struct{}{}
	}

	cachedMembers = []string{}
	for _, m := range newMembers {
		member := *m.Login
		if _, ok := committerSet[member]; !ok {
			cachedMembers = append(cachedMembers, member)
		}
	}

	fmt.Printf("Members: %v. Committers: %v.\n", cachedMembers, cachedCommitters)
	return cachedMembers, cachedCommitters
}
