const { Container, publicInternet } = require('@quilt/quilt');

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

  bot.allowFrom(publicInternet, 80);
  publicInternet.allowFrom(bot, 80);
  publicInternet.allowFrom(bot, 443);
  publicInternet.allowFrom(bot, 53);

  return bot;
};
