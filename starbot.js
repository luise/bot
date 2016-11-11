function deploy(slack_channel, slack_endpoint, github_oath) {
    service = new Service("bot", [new Container("quilt/star-bot").setEnv({
        "SLACK_CHANNEL": slack_channel,
        "SLACK_ENDPOINT": slack_endpoint,
        "GITHUB_OATH": github_oath,
    })]);

    service.connect(80, publicInternet);
    service.connect(53, publicInternet);

    var namespace = createDeployment({
        namespace: "quilt-star-bot",
        adminACL: ["local"],
    });

    var baseMachine = new Machine({
        provider: "Amazon",
        cpu: new Range(1),
        ram: new Range(1),
        sshKeys: githubKeys("ejj"),
    });

    // Boot VMs with the properties of `baseMachine`.
    namespace.deploy(baseMachine.asMaster());
    namespace.deploy(baseMachine.asWorker());
    namespace.deploy(service);
}

module.exports.deploy = deploy;
