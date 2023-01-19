# How to contribute to the Scalr CLI
## Basic steps
Here are the basic steps to make a change and contribute it back to the project.

1. [Fork](https://docs.github.com/en/get-started/quickstart/fork-a-repo) the [Scalr/scalr-cli](https://github.com/Scalr/scalr-cli) repo.
2. Make the changes and commit to your fork.
3. Create a [pull request](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/about-pull-requests).

## Development environment

We recommend using [VS Studio Code](https://code.visualstudio.com/) to edit the code. As the Scalr CLI tool is written in [Go](https://go.dev/), you will need to install Go somewhere in your PATH.

After making your code changes, simply run `go run .` to run the code directly or `go build -o scalr` to produce your own `scalr` binary. When running `scalr -version` on your custom binary, you should see version `0.0.0`, indicating that this is a local development version.

If you want to produce binaries for all supported operating systems and architectures, run `./build-all.sh`. All resulting binaries will then be found in the `./bin` directory.

## Rules

We try to make it as easy and painless as possible to contribute back to the project. However, some minimal rules must be followed to keep the project in good shape and maintain quality.

1. Please sign the [CLA](https://github.com/Scalr/scalr-cli/blob/CLA/Contribution_Agreement.md) and send it to support@scalr.com. This is required before we can merge any PRs.

2. Each pull request should contain only **one** bugfix/feature. If you want to add several features, please make individual pull requests for each one. Putting lots of changes in one single PR will make it harder for the maintainers to approve it, as **all** new changes will need to be tested and approved as a whole. Splitting it into individual requests makes it possible to approve some, while others can be pushed back for additional work.

3. Make sure that each commit has a **clear** and **complete** title and description on what has been fixed/changed/updated/added. As this will be used for building the release changelog, it's important that it's accurate and explains to the users what updating to this version will entail. Changes that breaks backwards compatibility is discouraged and needs a very strong reason to be approved.
