package main

import (
	"fmt"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"

	log "github.com/Sirupsen/logrus"
	"github.com/google/go-github/github"
	"github.com/nlopes/slack"
)

var (
	githubOwner       = "kelda"
	githubRepo        = "kelda"
	githubInstallRepo = "install"

	// Information about the Google sheet to update.
	googleSpreadsheetID    = "1Zj7lbFBO17h9yROxKwYSZ84QJhxjyxqy3bbh6NxDx88"
	visitorsSheetName      = "Github Daily Visitors"
	clonesKeldaSheetName   = "Github Daily Clones: Kelda"
	clonesInstallSheetName = "Github Daily Clones: Install"
	summarySheetName       = "Summary"

	// First row of the visitors and clones sheets that contains content.
	firstContentRow = 3

	// Format to use for dates written in the sheet.
	timeFormat = "1/2/2006"

	// errorValue is the value to use for a statistic when there's an error
	// fetching it.
	errorValue = -1
)

// recordMetrics records metrics about Kelda engagement in a Google spreadsheet.
// When this function encounters errors, it attempts to write an error value
// the spreadsheet, logs the error, and continues. We avoid exiting on error
// so that we can collect partial results when possible.
func recordMetrics(githubClient *github.Client, googleClient *sheets.Service,
	slackClient *slack.Client) {
	recordDailyGithubViews(githubClient, googleClient)
	recordDailyGithubClones(
		githubRepo, clonesKeldaSheetName, githubClient, googleClient)
	recordDailyGithubClones(
		githubInstallRepo, clonesInstallSheetName, githubClient, googleClient)
	recordTotalData(githubClient, googleClient, slackClient)
}

// newGoogleClient creates a new client to use to read and write Google Sheets data.
func newGoogleClient(secretJSON []byte) (*sheets.Service, error) {
	config, err := google.JWTConfigFromJSON(
		secretJSON, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		return nil, err
	}
	httpClient := config.Client(oauth2.NoContext)

	return sheets.New(httpClient)
}

// getPresentedToFormula returns a formula that calculates the total number
// of people presented to up to and including the date in column A of the
// given row.
func getPresentedToFormula(dateRowIndex int) string {
	var formulaFmt = `=SUMIF('Presentations'!A%[2]d:A, ` +
		`"<="&A%[1]d, 'Presentations'!D%[2]d:D)`
	return fmt.Sprintf(formulaFmt, dateRowIndex, firstContentRow)
}

// getDailyTotalFormula returns a formula that calculates the total of the entries
// in column C of the given sheet, up to and including the date in column A of
// destRow. The formula assumes that the given sheet has dates in column A.
func getDailyTotalFormula(sheetName string, destRow int) string {
	var formulaFmt = `=SUMIF('%[1]s'!A%[2]d:A, "<="&A%[3]d, ` +
		`'%[1]s'!C%[2]d:C)`
	return fmt.Sprintf(formulaFmt, sheetName, firstContentRow, destRow)
}

// updateDailyTrafficData updates sheetName with the traffic information in data
// that's not already in sheetName. The last entry in data is ignored, based on the
// assumption that it's from today so not yet a complete count.
func updateDailyTrafficData(
	googleClient *sheets.Service, sheetName string, data []*github.TrafficData) {
	// Read the most recent entry in the Daily Visitors tab in order to determine
	// which days are new and should be added.
	readRange := fmt.Sprintf("%s!A%d:E", sheetName, firstContentRow)
	readResp, err := googleClient.Spreadsheets.Values.Get(
		googleSpreadsheetID, readRange).Do()
	if err != nil {
		log.WithError(err).Warnf("unable to read data from %s", readRange)
		return
	}

	existingValues := len(readResp.Values)
	// Find the index of the first day of visit data that's not already in the
	// sheet.
	firstNew := 0
	if existingValues > 0 {
		mostRecent := readResp.Values[existingValues-1][0]
		for i, visit := range data {
			if visit.GetTimestamp().Format(timeFormat) == mostRecent {
				firstNew = i + 1
				break
			}
		}
	}

	// Write any new values.
	if firstNew < len(data)-1 {
		// Write the data beginning at firstNew, and not including the last
		// entry (which we assume is for today, so may not yet have the
		// complete count).
		write := sheets.ValueRange{}
		for _, v := range data[firstNew : len(data)-1] {
			r := []interface{}{
				v.GetTimestamp().Format(timeFormat),
				v.GetCount(),
				v.GetUniques(),
			}
			write.Values = append(write.Values, r)
		}
		start := firstContentRow + existingValues
		end := start + len(write.Values)
		writeRange := fmt.Sprintf("%s!A%d:C%d", sheetName, start, end)
		updateCall := googleClient.Spreadsheets.Values.Update(
			googleSpreadsheetID, writeRange, &write)
		_, err := updateCall.ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			errMsg := fmt.Sprintf(
				"unable to update range %s in sheet:", writeRange)
			log.WithError(err).Warnf(errMsg)
		}
	}
}

// recordDailyGithubViews gets per-day data about Github views from Github, and then
// writes that data to a Google Sheet.
func recordDailyGithubViews(githubClient *github.Client, googleClient *sheets.Service) {
	views, _, err := githubClient.Repositories.ListTrafficViews(
		ctx(), githubOwner, githubRepo, nil)
	if err == nil {
		updateDailyTrafficData(googleClient, visitorsSheetName, views.Views)
	} else {
		log.WithError(err).Warnf("error listing Github daily views")
	}
}

// recordDailyGithubClones gets per-day data about git clones from Github, and then
// writes that data to a Google Sheet.
func recordDailyGithubClones(
	repo string,
	sheetName string,
	githubClient *github.Client,
	googleClient *sheets.Service) {
	clones, _, err := githubClient.Repositories.ListTrafficClones(
		ctx(), githubOwner, repo, nil)
	if err == nil {
		updateDailyTrafficData(googleClient, sheetName, clones.Clones)
	} else {
		log.WithError(err).Warnf("error listing Github daily clones")
	}
}

func getTotalReleaseDownloads(githubClient *github.Client) int {
	releases, _, err := githubClient.Repositories.ListReleases(
		ctx(), githubOwner, githubRepo, nil)
	if err != nil {
		log.WithError(err).Warnf("unable to list github repositories")
		return errorValue
	}
	totalReleaseDownloads := 0
	for _, r := range releases {
		for _, a := range r.Assets {
			totalReleaseDownloads += a.GetDownloadCount()
		}
	}
	return totalReleaseDownloads
}

// getTotalCommits returns the total number of git commits in the main Kelda repo.
func getTotalCommits(githubClient *github.Client) int {
	// The github client paginates responses, so we need to iterate through all
	// of the pages to get the total number of commits. Unfortunately there is
	// not currently an API to directly get the total number of commits.
	n := 0
	opt := &github.CommitsListOptions{
		ListOptions: github.ListOptions{},
	}
	for {
		commits, resp, err := githubClient.Repositories.ListCommits(
			ctx(), githubOwner, githubRepo, opt)
		if err != nil {
			log.WithError(err).Warnf("unable to list git commits")
			return errorValue
		}
		n += len(commits)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return n
}

// getTotalContributors returns the total number of github contributors.
func getTotalContributors(githubClient *github.Client) int {
	contribs, _, err := githubClient.Repositories.ListContributorsStats(
		ctx(), githubOwner, githubRepo)
	if err != nil {
		log.WithError(err).Warnf(
			"unable to get total contributors from Github")
		return errorValue
	}
	return len(contribs)
}

// getSlackUsers returns the total number of Slack users.
func getSlackUsers(slackClient *slack.Client) int {
	users, err := slackClient.GetUsers()
	numUsers := 0
	if err != nil {
		log.WithError(err).Warnf("unable to get total number of Slack users")
		return errorValue
	}
	for _, u := range users {
		// Count only non-deleted, non-bot users. There's a bug with
		// the IsBot field so we need to check for slackbot directly.
		if !u.Deleted && !(u.Name == "slackbot") {
			numUsers++
		}
	}
	return numUsers
}

// getActiveSlackUsers returns the total number of active Slack users (as defined
// by Slack, which defines an active user as someone they'll bill for).
func getActiveSlackUsers(slackClient *slack.Client) int {
	users, err := slackClient.GetBillableInfoForTeam()
	if err != nil {
		log.WithError(err).Warnf(
			"unable to get total number of active Slack users")
		return errorValue
	}
	numActive := 0
	for _, status := range users {
		if status.BillingActive {
			numActive++
		}
	}
	return numActive
}

// recordTotalData collects a variety of statistics about total engagement with Kelda
// on Slack and Github, and writes it to a Google Sheet.
func recordTotalData(
	githubClient *github.Client,
	googleClient *sheets.Service,
	slackClient *slack.Client) {
	repoStats, _, err := githubClient.Repositories.Get(
		ctx(), githubOwner, githubRepo)
	githubStars := errorValue
	githubForks := errorValue
	if err == nil {
		githubStars = repoStats.GetStargazersCount()
		githubForks = repoStats.GetForksCount()
	} else {
		log.WithError(err).Warnf(
			"unable to get github repository summary statistics")
	}

	date := time.Now().Format(timeFormat)

	// Figure out where to write the data: if the last row in the spreadsheet
	// is from today, we should override it (in case any of the stats have
	// increased); otherwise, we should write a new row.
	readRange := fmt.Sprintf("%s!A1:A", summarySheetName)
	readResp, err := googleClient.Spreadsheets.Values.Get(
		googleSpreadsheetID, readRange).Do()
	if err != nil {
		log.WithError(err).Warnf(
			"unable to determine where in Spreadsheet to write data")
		return
	}
	destRow := len(readResp.Values)
	mostRecent := readResp.Values[destRow-1][0]
	if mostRecent != date {
		destRow++
	}

	data := [][]interface{}{[]interface{}{
		date,
		getPresentedToFormula(destRow),
		getDailyTotalFormula(visitorsSheetName, destRow),
		githubStars,
		getSlackUsers(slackClient),
		getActiveSlackUsers(slackClient),
		getDailyTotalFormula(clonesKeldaSheetName, destRow),
		getTotalReleaseDownloads(githubClient),
		getDailyTotalFormula(clonesInstallSheetName, destRow),
		getTotalContributors(githubClient),
		getTotalCommits(githubClient),
		githubForks,
	}}
	write := sheets.ValueRange{Values: data}

	// Write the data to the sheet.
	writeRange := fmt.Sprintf("%s!A%d:L%d", summarySheetName, destRow, destRow)
	u := googleClient.Spreadsheets.Values.Update(
		googleSpreadsheetID, writeRange, &write)
	_, err = u.ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		log.WithError(err).Warnf(fmt.Sprintf(
			"unable to update range %s in sheet", writeRange))
	}
}
