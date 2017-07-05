const {Container, Service, publicInternet} = require("@quilt/quilt");

exports.New = function(github_oath, google_json, slack_token) {
    service = new Service("bot", [
        new Container("quilt/bot")
            .withEnv({
                "GITHUB_OATH": github_oath,
                "SLACK_TOKEN": slack_token,
            }).withFiles({
                "google_secret.json": google_json
            })
    ]);

    publicInternet.connect(80, service);
    service.connect(80, publicInternet);
    service.connect(443, publicInternet);
    service.connect(53, publicInternet);

    return service;
}
