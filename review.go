package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/go-github/github"
)

type review struct {
	State string
	User  github.User
}

func runReview(client *github.Client) {
	repos, _, err := client.Repositories.ListByOrg(ctx(), "kelda", nil)
	if err != nil {
		log.Println("Failed to list repos: ", err)
		return
	}

	for _, repo := range repos {
		prs, _, err := client.PullRequests.List(ctx(), "kelda", *repo.Name, nil)
		if err != nil {
			log.Println("Failed to list pull requests: ", err)
			return
		}

		for _, pr := range prs {
			processPullRequest(client, pr)
		}
	}
}

var memberIndex = rand.Intn(100)
var committerIndex = rand.Intn(100)

func userInList(user *string, listOfUsers []string) bool {
	for _, userInList := range listOfUsers {
		if userInList == *user {
			return true
		}
	}
	return false
}

func processPullRequest(client *github.Client, pr *github.PullRequest) {
	log.Printf("Processing PR %d\n", *pr.Number)
	members, committers := getTeamMembers(client)

	// Return if there are any reviewers who have been assigned but who
	// haven't done anything yet.
	reviewers, err := getRequestedReviewers(client, pr)
	if err != nil {
		log.Println("Failed to list requested reviewers: ", err)
		return
	}
	if len(reviewers) > 0 {
		log.Printf("PR %d has %d outstanding reviewers\n",
			*pr.Number, len(reviewers))
		return
	}

	// Determine what reviews have already occurred. This list will include
	// reviews that added comments, reviews that requested changes, and
	// reviews that approved the PR.
	reviews, err := getReviews(client, pr)
	if err != nil {
		log.Println("Failed to list reviews: ", err)
		return
	}

	if len(reviews) == 0 {
		// The pull request has had no reviews, so assign a reviewer.
		assignReviewer(client, pr, members, &memberIndex)
		return
	}

	// Parse the reviews to determine whether a second person needs to be assigned
	// to do a review.
	nonCommitterApproved := false
	committerReviewedAfterApproval := false
	for _, review := range reviews {
		reviewerIsCommitter := userInList(review.User.Login, committers)
		if nonCommitterApproved && reviewerIsCommitter {
			// This code relies on the property that reviews
			// are returned in chronological order.
			// We only care about committer reviews that happen
			// after the non-committer approval, because we want
			// to ping a committer to merge the PR after the
			// non-committer approval even if they've already
			// looked at it earlier.
			committerReviewedAfterApproval = true
		}
		if review.State == "APPROVED" && !reviewerIsCommitter {
			nonCommitterApproved = true
		}
	}

	prByCommitter := userInList(pr.User.Login, committers)
	if nonCommitterApproved && !committerReviewedAfterApproval &&
		!prByCommitter {
		// A committer hasn't yet been involved in this pull request, so assign
		// one.
		assignReviewer(client, pr, committers, &committerIndex)
	}
	// Either there's an in-process review (e.g., a non-committer has done
	// a review but not approved it yet), in which case we don't need to
	// assign anyone else yet, or a committer has seen the PR, so no one
	// else needs to review it.
}

func assignReviewer(client *github.Client, pr *github.PullRequest,
	reviewerOptions []string, index *int) {
	reviewer := ""

	// Choose a reviewer from the list, who isn't the author of the PR.
	for i := 0; i < len(reviewerOptions); i++ {
		*index++
		possibleReviewer := reviewerOptions[*index%len(reviewerOptions)]
		if possibleReviewer != *pr.User.Login {
			reviewer = possibleReviewer
			break
		}
	}
	if reviewer == "" {
		log.Printf("No potential reviewers for PR %d\n", *pr.Number)
		return
	}

	log.Printf("Assigning pull request %d review to %s\n", *pr.Number, reviewer)
	post := map[string][]string{
		"reviewers": []string{reviewer},
	}
	err := prRequest(client, pr, "POST", "requested_reviewers", &post, nil)
	if err != nil {
		log.Printf("Failed to assign %s to PR %d: %s\n",
			reviewer, *pr.Number, err)
	}
}

// getRequestedReviewers returns people from whom a review has been requested,
// and who haven't done anything yet (i.e., they haven't approved the PR,
// or left comments in a review).
func getRequestedReviewers(client *github.Client,
	pr *github.PullRequest) ([]github.User, error) {

	var result struct{ Users []github.User }
	err := prRequest(client, pr, "GET", "requested_reviewers", nil, &result)
	return result.Users, err
}

func getReviews(client *github.Client, pr *github.PullRequest) ([]review, error) {
	var reviews []review
	err := prRequest(client, pr, "GET", "reviews", nil, &reviews)
	return reviews, err
}

func prRequest(client *github.Client, pr *github.PullRequest, method,
	action string, post, result interface{}) error {

	url := fmt.Sprintf("/repos/kelda/%s/pulls/%d/%s", *pr.Base.Repo.Name, *pr.Number, action)
	req, err := client.NewRequest(method, url, post)
	if err != nil {
		return err
	}

	_, err = client.Do(ctx(), req, result)
	return err
}

// cachedMembers and cachedCommiters contain a cached copy of all of the members of the
// Kelda team (including the committers) and of all of the Kelda committers,
// respectively.
var cachedMembers, cachedCommitters []string
var memberRateLimit = time.Tick(time.Hour)

// getTeamMembers returns two lists: the first list is of all of the members of
// the Kelda team, including the committers, and the second list is of only the
// committers in the team.
func getTeamMembers(client *github.Client) (members, committers []string) {
	select {
	case <-memberRateLimit:
	default:
		if cachedMembers != nil && cachedCommitters != nil {
			return cachedMembers, cachedCommitters
		}
	}

	teams, _, err := client.Organizations.ListTeams(ctx(), "kelda", nil)
	if err != nil {
		log.Println("Failed to list teams: ", err)
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
		log.Println("Failed to list team members: ", err)
		return cachedMembers, cachedCommitters
	}

	newCommitters, _, err := client.Organizations.ListTeamMembers(ctx(), committerID, nil)
	if err != nil {
		log.Println("Failed to list committers: ", err)
		return cachedMembers, cachedCommitters
	}

	cachedCommitters = []string{}
	for _, c := range newCommitters {
		committer := *c.Login
		cachedCommitters = append(cachedCommitters, committer)
	}

	cachedMembers = []string{}
	for _, m := range newMembers {
		member := *m.Login
		cachedMembers = append(cachedMembers, member)
	}

	return cachedMembers, cachedCommitters
}
