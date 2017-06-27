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
	repos, _, err := client.Repositories.ListByOrg(ctx(), "quilt", nil)
	if err != nil {
		fmt.Println("Failed to list repos: ", err)
		return
	}

	for _, repo := range repos {
		prs, _, err := client.PullRequests.List(ctx(), "quilt", *repo.Name, nil)
		if err != nil {
			fmt.Println("Failed to list pull requests: ", err)
			return
		}

		for _, pr := range prs {
			processPullRequest(client, pr)
		}
	}
}

var memberIndex = rand.Intn(100)
var committerIndex = rand.Intn(100)

func processPullRequest(client *github.Client, pr *github.PullRequest) {
	members, committers := getTeamMembers(client)

	reviewers, err := getRequestedReviewers(client, pr)
	if err != nil {
		fmt.Println("Failed to list requested reviewers: ", err)
		return
	}

	reviews, err := getReviews(client, pr)
	if err != nil {
		fmt.Println("Failed to list reviews: ", err)
		return
	}

	// GitHub automatically removes the requested reviewer after they submit a review.
	// When this has happened, or if there is a pending review request, we shouldn't
	// assign someone else.
	// Note, this means, the PR will have no requested reviewer after the initial review
	// is submitted by the requested person or if someone submits a review before
	// quilt-bot submits a request. Either way, the PR will have been reviewed.
	if len(reviewers) > 0 || len(reviews) > 0 {
		return
	}

	var byCommitter bool
	for _, c := range committers {
		if c == *pr.User.Login {
			byCommitter = true
		}
	}

	var assignee string
	if byCommitter {
		assignee = chooseReviewer(members, &memberIndex, pr)
	} else {
		assignee = chooseReviewer(committers, &committerIndex, pr)
	}

	if assignee == "" {
		return
	}

	if err := assignRequestedReviewer(client, pr, assignee); err != nil {
		fmt.Printf("Failed to assign %s to PR %d: %s\n",
			assignee, *pr.Number, err)
	}
}

func chooseReviewer(options []string, index *int, pr *github.PullRequest) string {
	if len(options) > 0 {
		*index++
		return options[*index%len(options)]
	}
	return ""
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

	return prRequest(client, pr, "POST", "requested_reviewers", &post, nil)
}

func getReviews(client *github.Client, pr *github.PullRequest) ([]review, error) {
	var reviews []review
	err := prRequest(client, pr, "GET", "reviews", nil, &reviews)
	return reviews, err
}

func prRequest(client *github.Client, pr *github.PullRequest, method,
	action string, post, result interface{}) error {

	url := fmt.Sprintf("/repos/quilt/%s/pulls/%d/%s", *pr.Base.Repo.Name, *pr.Number, action)
	req, err := client.NewRequest(method, url, post)
	if err != nil {
		return err
	}

	_, err = client.Do(ctx(), req, result)
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

	teams, _, err := client.Organizations.ListTeams(ctx(), "quilt", nil)
	if err != nil {
		fmt.Println("Failed to list teams: ", err)
		return cachedMembers, cachedCommitters
	}

	var memberID, committerID int
	for _, team := range teams {
		switch *team.Name {
		case "Reviewers":
			memberID = *team.ID
		case "Committers":
			committerID = *team.ID
		}
	}

	newMembers, _, err := client.Organizations.ListTeamMembers(ctx(), memberID, nil)
	if err != nil {
		fmt.Println("Failed to list team members: ", err)
		return cachedMembers, cachedCommitters
	}

	newCommitters, _, err := client.Organizations.ListTeamMembers(ctx(), committerID, nil)
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
