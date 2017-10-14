const { Container, publicInternet } = require('@quilt/quilt');

/**
 * Creates a container running the Kelda bot. Callers must explicitly allow
 * traffic from the public internet if they want it to be publicly accessible.
 */
exports.New = function New(githubOauth, googleJson, slackToken) {
  const bot = new Container('bot', 'quilt/bot', {
    env: {
      GITHUB_OAUTH: githubOauth,
      SLACK_TOKEN: slackToken,
    },
    filepathToContent: {
      '/go/src/app/google_secret.json': googleJson,
    },
  });

  publicInternet.allowFrom(bot, 80);
  publicInternet.allowFrom(bot, 443);
  publicInternet.allowFrom(bot, 53);

  return bot;
};
