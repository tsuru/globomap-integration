# Tsuru integration with [globomap](https://github.com/globocom/globomap-api)

[![Build Status](https://travis-ci.org/tsuru/globomap-integration.svg?branch=master)](https://travis-ci.org/tsuru/globomap-integration)

## Configuration

Required environment variables:

- `GLOBOMAP_LOADER_HOSTNAME`: API used to post updates
- `GLOBOMAP_API_HOSTNAME`: API used to search for comp units
- `GLOBOMAP_USERNAME`: Username used to authenticate with globomap
- `GLOBOMAP_PASSWORD`: Password used to authenticate with globomap
- `TSURU_HOSTNAME`: tsuru API, used to check for information about apps, pools and nodes
- `TSURU_TOKEN`: token used in tsuru API

## Running

This program can be run in three ways:

### Update mode

This is the default mode. Checks for new events about apps or pools and post them to globomap API. The time period can be set with `--start/-s` flag:

```
# Checks for events in the last 2 days
globomap-integration --start 2d
```

The time period can be set in days (`d`), hours (`h`) or minutes (`m`). The default value is 24 hours.

### Repeat mode

Run in the update mode, repeating with a specific frequency. To run in repeat mode, use `--repeat/-r` flag with the desired frequency:

```
# Repeats every 6 hours
globomap-integration --repeat 6h
```

In repeat mode, queries to globomap API that don't find results are retried by default. You can set two optional environment variables to configure the retry params:

- `RETRY_SLEEP_TIME`: sleep time between retries; defaults to 5 minutes
- `MAX_RETRIES`: maximum number of retries for each query; defaults to 20

### Load mode

Loads information about all apps, pools and nodes. To run in load mode, use `--load/-l` flag:

```
# Runs in load mode
globomap-integration --load
```

## Dry mode

Every running mode supports dry mode. With `--dry/-d` flag, the payload will be written to stdout, instead of posted to globomap loader API:

```
# Checks for events in the last 15 minutes and writes the payload to stdout
globomap-integration --start 15m --dry
```

## Verbose mode

For more output when running the program, add the `--verbose/-v` flag.
