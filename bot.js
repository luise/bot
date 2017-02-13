exports.New = function(slack_channel, slack_endpoint, github_oath) {
    service = new Service("bot", [new Container("quilt/bot").withEnv({
        "SLACK_CHANNEL": slack_channel,
        "SLACK_ENDPOINT": slack_endpoint,
        "GITHUB_OATH": github_oath,
    })]);

    service.connect(80, publicInternet);
    service.connect(443, publicInternet);
    service.connect(53, publicInternet);

    return service;
}
