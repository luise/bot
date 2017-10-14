package main

import (
	"golang.org/x/net/context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
	"github.com/nlopes/slack"
)

func main() {
	log.Println("Started!")

	// Initialize the various clients so we can re-use them.
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_OAUTH")},
	)
	tc := oauth2.NewClient(ctx(), ts)

	githubClient := github.NewClient(tc)

	gsName := "google_secret.json"
	gs, err := ioutil.ReadFile(gsName)
	if err != nil {
		log.Fatalf("Unable to read Google secret from %s: %s", gsName, err)
	}
	googleClient, err := newGoogleClient(gs)
	if err != nil {
		log.Fatalf("Unable to initialize Google client: %s", err)
	}

	st := os.Getenv("SLACK_TOKEN")
	slackClient := slack.New(st)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch github.WebHookType(r) {
		case "pull_request_review", "pull_request":
			runReview(githubClient)
		}
	})
	go http.ListenAndServe(":80", nil)

	reviewTicker := time.Tick(10 * time.Minute)

	// The metrics only need to be updated once a day, but do it twice
	// a day just to be safe (e.g., for the case Kelda bot gets restarted, to
	// make sure we don't miss a day of data).
	metricsTicker := time.Tick(12 * time.Hour)

	for {
		select {
		case <-reviewTicker:
			runReview(githubClient)
		case <-metricsTicker:
			recordMetrics(githubClient, googleClient, slackClient)
		}
	}
}

func ctx() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	return ctx
}
