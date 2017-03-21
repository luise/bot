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

	reviews, err := getReviews(client, pr)
	if err != nil {
		fmt.Println("Failed to check for approval: ", err)
		return
	}

	var approved bool
	users := map[string]struct{}{}
	for _, review := range reviews {
		users[*review.User.Login] = struct{}{}
		approved = approved || review.State == "APPROVED"
	}

	committerSet := map[string]struct{}{}
	for _, c := range committers {
		committerSet[c] = struct{}{}
		if _, ok := users[c]; ok {
			return
		}
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
		user := *reviewer.Login
		users[user] = struct{}{}
		if _, ok := committerSet[user]; ok {
			// Assigned to a comitter, nothing more to do.
			return
		}
	}

	var assignee string
	if len(users) == 0 {
		// No one is assigned to the PR, go ahead and pick someone.
		assignee = chooseReviewer(members, &memberIndex, pr)
	} else if approved {
		// No committer assigned, but the PR has been approved.
		assignee = chooseReviewer(committers, &committerIndex, pr)
	} else {
		return
	}

	if err := assignRequestedReviewer(client, pr, assignee); err != nil {
		fmt.Printf("Failed to assign %s to PR %d: %s\n",
			assignee, *pr.Number, err)
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

func prRequest(client *github.Client, pr *github.PullRequest, method,
	action string, post, result interface{}) error {

	url := fmt.Sprintf("/repos/quilt/%s/pulls/%d/%s", *pr.Base.Repo.Name, *pr.Number, action)
	req, err := client.NewRequest(method, url, post)
	if err != nil {
		return err
	}

	// This API isn't ready yet, so we have to disclaim with a magic header.
	req.Header.Set("Accept", "application/vnd.github.black-cat-preview+json")

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
