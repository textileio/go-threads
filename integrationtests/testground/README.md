Here contains test plans to be run on [Testground](https://docs.testground.ai/).

# Quick Start

```
git clone https://github.com/testground/testground.git
cd testground
# compile Testground and all related dependencies
make install
# start the Testground daemon, listening by default on localhost:8042
testground daemon

# in another terminal window
testground plan import --from ../testground
# then you can run the individual test cases. Below runs the `sync-threads` test case in Docker with 10 instances
testground run single --wait -p testground -t sync-threads -b docker:go -r local:docker -i 10
```

You could pass test parameters in command line. For example, below builds the test as native executables, runs 2 instances, with debug log printed to the console. See manifest.toml for all test parameters.
```
testground run single --wait -p testground -t sync-threads -b exec:go -r local:exec -i 2 -tp debug=true
```
