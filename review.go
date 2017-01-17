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

func processPullRequest(client *github.Client, pr *github.PullRequest) {
	var assignees []string
	for _, user := range pr.Assignees {
		if *user.Login == "ejj" {
			// The pull request has already been assigned to Ethan, so
			// nothing more for quilt-bot to do.
			return
		}
		assignees = append(assignees, *user.Login)
	}

	var assignee string
	if len(assignees) == 0 {
		reviewer := chooseReviewer(client, pr)
		if reviewer != nil {
			assignee = *reviewer.Login
		}
	} else {
		reviews, err := getReviews(client, pr)
		if err != nil {
			fmt.Println("Failed to check for approval: ", err)
			return
		}

		for _, review := range reviews {
			if review.State == "APPROVED" {
				assignee = "ejj"
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

var roundRobinIndex = rand.Intn(100)

func chooseReviewer(client *github.Client, pr *github.PullRequest) *github.User {
	members := getTeamMembers(client)
	for i := 0; i < len(members); i++ {
		roundRobinIndex++
		member := members[roundRobinIndex%len(members)]
		if *member.Login != "ejj" && *member.Login != *pr.User.Login {
			return member
		}
	}

	return nil
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

var cachedMembers []*github.User
var memberRateLimit = time.Tick(time.Hour)

func getTeamMembers(client *github.Client) []*github.User {
	var refresh bool
	select {
	case <-memberRateLimit:
		refresh = true
	default:
		refresh = false
	}

	if !refresh && cachedMembers != nil {
		return cachedMembers
	}

	teams, _, err := client.Organizations.ListTeams("Netsys", nil)
	if err != nil {
		fmt.Println("Failed to list teams: ", err)
		return cachedMembers
	}

	var quiltID *int
	for _, team := range teams {
		if *team.Name == "Quilt" {
			quiltID = team.ID
			break
		}
	}

	members, _, err := client.Organizations.ListTeamMembers(*quiltID, nil)
	if err != nil {
		fmt.Println("Failed to list team members: ", err)
		return cachedMembers
	}

	cachedMembers = members
	return cachedMembers
}
