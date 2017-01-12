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
		processPullRequest(client, getTeamMembers(client), pr)
	}
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

func processPullRequest(client *github.Client, members []*github.User,
	pr *github.PullRequest) {

	users := map[string]struct{}{}
	approved := false

	reviews, err := getReviews(client, pr)
	if err != nil {
		fmt.Println("Failed to check for approval: ", err)
		return
	}
	for _, review := range reviews {
		users[*review.User.Login] = struct{}{}
		approved = approved || review.State == "APPROVED"
	}

	// Github has a rather aggressive API rate limit at 5000 requests per hour.  Thus
	// we go to rather great lengths to just have to make the single `getReviews`
	// request without having to go one to get the requested reviewers as well.

	// Ethan reviewed this pull request, nothing more to do.
	if _, ok := users["ejj"]; ok {
		return
	}

	// Someone has been assigned this PR, but they haven't approved it yet.  Can't do
	// anything until they final say it's OK.
	if len(users) > 0 && !approved {
		return
	}

	reviewers, err := getRequestedReviewers(client, pr)
	if err != nil {
		fmt.Println("Failed to list requested reviewers: ", err)
		return
	}

	for _, reviewer := range reviewers {
		users[*reviewer.Login] = struct{}{}
	}

	var assignment string
	if _, ok := users["ejj"]; !ok && approved {
		if *pr.User.Login == "ejj" {
			return
		}
		assignment = "ejj"
	} else if len(users) == 0 {
		// No one is assigned to the PR, go ahead and pick someone.
		reviewer := chooseReviewer(pr, members)
		if reviewer == nil {
			fmt.Println("Failed to choose reviewer for PR: ", *pr.Number)
			return
		}
		assignment = *reviewer.Login
	} else {
		return
	}

	if err := assignRequestedReviewer(client, pr, assignment); err != nil {
		fmt.Printf("Failed to assign %s to PR %d: %s\n",
			assignment, *pr.Number, err)
	}
}

var roundRobinIndex = rand.Intn(100)

func chooseReviewer(pr *github.PullRequest, members []*github.User) *github.User {
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

func getRequestedReviewers(client *github.Client,
	pr *github.PullRequest) ([]github.User, error) {

	var result []github.User
	err := prRequest(client, pr, "GET", "requested_reviewers", nil, &result)
	return result, err
}

func assignRequestedReviewer(client *github.Client, pr *github.PullRequest,
	login string) error {
	fmt.Printf("Assign Pull Request %d review to %s\n", *pr.Number, login)

	post := map[string][]string{
		"reviewers": []string{login},
	}
	err := prRequest(client, pr, "POST", "requested_reviewers", &post, nil)
	return err
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
