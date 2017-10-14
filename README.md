# Kelda Bot

Handles Code Review and Slack notifications.

## Configuring The Code Review Webhook

We use webhooks instead of polling to trigger review processing so that we don't
get rate limited. A single webhook can be added to the entire organization, rather
than to each individual repository.

To configure:
1. Boot the kelda-bot spec and note its public IP
2. Navigate to your organization page (e.g. github.com/kelda)
3. Click "Settings"
4. Click "Webhooks"
5. The "Secret" can be left blank as it is unused
6. Enter `http://${KELDA_BOT_PUBLIC_IP}` under `Payload URL`
7. Set the Webhook trigger to `Send me everything`
8. Click "Add webhook"
