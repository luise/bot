package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
)

type message struct {
	Title string `json:"title"`
	Short bool   `json:"short"`
	Value string `json:"value"`
}

type slackPost struct {
	Channel   string    `json:"channel"`
	Color     string    `json:"color"`
	Fields    []message `json:"fields"`
	Pretext   string    `json:"pretext"`
	Username  string    `json:"username"`
	Iconemoji string    `json:"icon_emoji"`
}

// Post to slack.
func slack(hookurl string, p slackPost) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}

	resp, err := http.Post(hookurl, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t, _ := ioutil.ReadAll(resp.Body)
		return errors.New(string(t))
	}

	return nil
}
