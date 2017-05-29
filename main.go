package main

import (
	"fmt"
	"golang.org/x/net/context"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
)

func main() {
	fmt.Println("Started!")

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_OATH")},
	)
	tc := oauth2.NewClient(ctx(), ts)

	client := github.NewClient(tc)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch github.WebHookType(r) {
		case "pull_request_review", "pull_request":
			runReview(client)
		}
	})
	go http.ListenAndServe(":80", nil)

	go func() {
		for range time.Tick(10 * time.Minute) {
			runReview(client)
		}
	}()
}

func ctx() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	return ctx
}
