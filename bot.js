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

  service.allowFrom(publicInternet, 80);
  publicInternet.allowFrom(service, 80);
  publicInternet.allowFrom(service, 443);
  publicInternet.allowFrom(service, 53);

  return service;
};
