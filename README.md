# url-shortener
An URL shortener using named URLs.

Tests [![Build status](https://circleci.com/gh/pcorpet/url-shortener.png?circle-token=d1e30f1ceff91832351b06e0433cebfa47409de9)](https://circleci.com/gh/pcorpet/url-shortener)

This is an implementation of an equivalent of go/ used inside Google. Read about it by [Kevin Burke](https://kev.inburke.com/kevin/url-shortener/).

## Build

To create clean binaries, we use Docker. Install docker, then run

```
./build.sh.
```

It will generate a docker image `lascap/url-shortener` that you can deploy.

## Configuration

The following env variables are used:
* `PORT`: defines the port on which to listen. Defaults to 5000.
* `MONGODB_URL`: the URL to connect to MongoDB. Format: `[mongodb://][user:pass@]host1[:port1][,host2[:port2],...][/database][?options]`
* `SHORT_URL_PREFIX`: An URL prefix to display nicer URLs if you have a rewriter enabled, e.g. `http://go/`.

## Setup

Once deployed on a server, we recommend that your users automatically redirect even shorter links to the server. Here is the setup I use:

* Have each user install [Requestly](https://chrome.google.com/webstore/detail/requestly/mdnleldcmiljblolnjhpnblkcekpdkpa).
* Make them add a rule "Replace Host", where they replace `http://go/` by `http://URL-of-your-server.com/`.

With this setup you can have very-easy-to-remember links to important documents and pages.
