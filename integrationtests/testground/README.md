Here contains test plans to be run on [Testground](https://docs.testground.ai/).

# Setup

```
git clone https://github.com/testground/testground.git
cd testground
#  compile Testground and all related dependencies
make install
#  start the Testground daemon, listening by default on localhost:8042
testground daemon
```

Run below from another terminal window. This will add tests here to the testground home directory, which is `$HOME/testground` by default, unless you explicitly define `$TESTGROUND_HOME`.
```
testground plan import --from . --name go-threads
```

# Run

Then you can run the prepared composition plans as follows:
```
#  This one sticks to an early version of go-threads
testground run composition --wait -f baseline-docker.toml
#  This one always test against the current head of go-threads
testground run composition --wait -f head-docker.toml
#  This one uses whatever version specified by go.mod in the current directory. Do check/update dependencies before running this.
testground run composition --wait -f current-docker.toml
```

You can also run individual test case and pass test parameters in command line. For example, below builds the test as native executables, runs 2 instances, with debug log printed to the console. See manifest.toml for all test parameters.
```
testground run single --wait -p go-threads -t sync-threads -b exec:go -r local:exec -i 2 -tp debug=true
```

# Analysize

1. Check for errors running the tests
1. Check the recorded metrics and compare them with a baseline run
```
cd $HOME/testground/data/outputs/local_docker/go-threads/<runner-id>
find . -name "results.out" | xargs grep elapsed-seconds | sort -t ':' -k 4
```


# Develop

To run these tests after making some changes to go-threads, do the following:

1. Commit you changes and push to a branch
1. `GOPRIVATE=* go get github.com/textileio/go-threads@<your branch>`
1. `testground run composition --wait -f current-docker.toml` and have a look at the results (See [section Analysize](#analysize))
1. Commit changes to the branch
