const { Container, publicInternet, allowTraffic } = require('kelda');

/**
 * Creates a container running the Kelda bot. Callers must explicitly allow
 * traffic from the public internet if they want it to be publicly accessible.
 */
exports.New = function New(githubOauth, googleJson, slackToken) {
  const bot = new Container('bot', 'keldaio/bot', {
    env: {
      GITHUB_OAUTH: githubOauth,
      SLACK_TOKEN: slackToken,
    },
    filepathToContent: {
      '/go/src/app/google_secret.json': googleJson,
    },
  });

  allowTraffic(bot, publicInternet, 80);
  allowTraffic(bot, publicInternet, 443);
  allowTraffic(bot, publicInternet, 53);

  return bot;
};
