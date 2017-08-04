const { Container, Service, publicInternet } = require('@quilt/quilt');

exports.New = function New(githubOauth, googleJson, slackToken) {
  const service = new Service('bot', [
    new Container('quilt/bot')
      .withEnv({
        GITHUB_OAUTH: githubOauth,
        SLACK_TOKEN: slackToken,
      }).withFiles({
        '/go/src/app/google_secret.json': googleJson,
      }),
  ]);

  publicInternet.connect(80, service);
  service.connect(80, publicInternet);
  service.connect(443, publicInternet);
  service.connect(53, publicInternet);

  return service;
};
