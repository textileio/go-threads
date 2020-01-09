# go-threads

[![Made by Textile](https://img.shields.io/badge/made%20by-Textile-informational.svg?style=popout-square)](https://textile.io)
[![Chat on Slack](https://img.shields.io/badge/slack-slack.textile.io-informational.svg?style=popout-square)](https://slack.textile.io)
[![GitHub license](https://img.shields.io/github/license/textileio/go-threads.svg?style=popout-square)](./LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/textileio/go-threads?style=flat-square)](https://goreportcard.com/report/github.com/textileio/go-threads?style=flat-square)
[![GitHub action](https://github.com/textileio/go-threads/workflows/Tests/badge.svg?style=popout-square)](https://github.com/textileio/go-threads/actions)
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=popout-square)](https://github.com/RichardLitt/standard-readme)

> Textile's threads implementation in Go

Go to [the docs](https://docs.textile.io/) for more about Textile.

Join us on our [public Slack channel](https://slack.textile.io/) for news, discussions, and status updates. [Check out our blog](https://medium.com/textileio) for the latest posts and announcements.

## Table of Contents

-   [Install](#install)
-   [Usage](#usage)
-   [Contributing](#contributing)
-   [Changelog](#changelog)
-   [License](#license)

## Install

    go get github.com/textileio/go-threads

## Usage

Go to https://godoc.org/github.com/textileio/go-threads.

## Libraries

> The following includes information about libraries built using go-threads.

| Name | Status | Platforms | Description |
| ---------|---------|---------|--------- |
| **Thread Clients** |
| [`js-threads-client`](//github.com/textileio/js-threads-client) | [![Threads version](https://img.shields.io/badge/dynamic/json.svg?style=popout-square&color=3527ff&label=go-threads&prefix=v&query=%24.dependencies%5B%27%40textile%2Fthreads-client-grpc%27%5D.version&url=https%3A%2F%2Fraw.githubusercontent.com%2Ftextileio%2Fjs-threads-client%2Fmaster%2Fpackage-lock.json)](https://github.com/textileio/go-threads) [![Build status](https://img.shields.io/github/workflow/status/textileio/js-threads-client/lint_test/master.svg?style=popout-square)](https://github.com/textileio/js-threads-client/actions?query=branch%3Amaster) | [![node](https://img.shields.io/badge/nodejs-blueviolet.svg?style=popout-square)](https://github.com/textileio/js-threads-client) [![web](https://img.shields.io/badge/web-blueviolet.svg?style=popout-square)](https://github.com/textileio/js-threads-client) [![react native](https://img.shields.io/badge/react%20native-blueviolet.svg?style=popout-square)](https://github.com/textileio/js-threads-client) | A JavaScript client for threads daemon. |
| [`dart-threads-client`](//github.com/textileio/dart-threads-client) | [![Threads version](https://img.shields.io/badge/dynamic/yaml?style=popout-square&color=3527ff&label=go-threads&prefix=v&query=packages.threads_client_grpc.version&url=https%3A%2F%2Fraw.githubusercontent.com%2Ftextileio%2Fdart-threads-client%2Fmaster%2Fpubspec.lock)](https://github.com/textileio/go-threads) [![Build status](https://img.shields.io/github/workflow/status/textileio/dart-threads-client/test/master.svg?style=popout-square)](https://github.com/textileio/dart-threads-client/actions?query=branch%3Amaster) | [![dart](https://img.shields.io/badge/dart-blueviolet.svg?style=popout-square)](https://github.com/textileio/dart-threads-client) [![flutter](https://img.shields.io/badge/flutter-blueviolet.svg?style=popout-square)](https://github.com/textileio/dart-threads-client) | A Dart client for threads daemon. |
| **Examples** |
| [`go-foldersync`](//github.com/textileio/go-foldersync) | [![Threads version](https://img.shields.io/github/v/release/textileio/go-threads?color=3529ff&label=go-threads&style=popout-square)](https://github.com/textileio/go-threads) [![Build status](https://img.shields.io/github/workflow/status/textileio/go-foldersync/Tests/master.svg?style=popout-square)](https://github.com/textileio/js-threads-client/actions?query=branch%3Amaster) | [![go-threads](https://img.shields.io/badge/golang-blueviolet.svg?style=popout-square)](https://github.com/textileio/go-foldersync) | An e2e demo to sync data between two golang clients. |
| [`js-foldersync`](//github.com/textileio/js-foldersync) | [![Threads version](https://img.shields.io/badge/dynamic/json.svg?style=popout-square&color=3527ff&label=go-threads&prefix=v&query=%24.dependencies%5B%27%40textile%2Fthreads-client-grpc%27%5D.version&url=https%3A%2F%2Fraw.githubusercontent.com%2Ftextileio%2Fjs-foldersync%2Fmaster%2Fpackage-lock.json)](https://github.com/textileio/go-threads) [![Build status](https://img.shields.io/github/workflow/status/textileio/js-foldersync/Test/master.svg?style=popout-square)](https://github.com/textileio/js-foldersync/actions?query=branch%3Amaster) | [![web](https://img.shields.io/badge/web-blueviolet.svg?style=popout-square)](https://github.com/textileio/js-foldersync) | A demo of writing and reading models with the js-threads-client. |

## Contributing

This project is a work in progress. As such, there's a few things you can do right now to help out:

-   **Ask questions**! We'll try to help. Be sure to drop a note (on the above issue) if there is anything you'd like to work on and we'll update the issue to let others know. Also [get in touch](https://slack.textile.io) on Slack.
-   **Open issues**, [file issues](https://github.com/textileio/go-threads/issues), submit pull requests!
-   **Perform code reviews**. More eyes will help a) speed the project along b) ensure quality and c) reduce possible future bugs.
-   **Take a look at the code**. Contributions here that would be most helpful are **top-level comments** about how it should look based on your understanding. Again, the more eyes the better.
-   **Add tests**. There can never be enough tests.

Before you get started, be sure to read our [contributors guide](./CONTRIBUTING.md) and our [contributor covenant code of conduct](./CODE_OF_CONDUCT.md).

## Changelog

[Changelog is published to Releases.](https://github.com/textileio/go-threads/releases)

## License

[MIT](LICENSE)
