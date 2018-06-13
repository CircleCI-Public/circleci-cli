# circleci-cli [![CircleCI](https://circleci.com/gh/circleci/circleci-cli.svg?style=svg&circle-token=8adb850cb9110a49ab0990f198dd78966f9e6073)](https://circleci.com/gh/circleci/circleci-cli) [![codecov](https://codecov.io/gh/circleci/circleci-cli/branch/master/graph/badge.svg?token=VJMF7kG1Om)](https://codecov.io/gh/circleci/circleci-cli)

This project is the seed for CircleCI's new command-line application.

## Requirements

* Go 1.10+
* Make
* ...

## Getting Started

You should already have [installed Go](https://golang.org/doc/install) and setup your [workspace](https://golang.org/doc/code.html#Workspaces).

This includes setting a valid `$GOPATH`.

### 1. Get the repo

TODO: make this easier once repo is public

```
# Setup circleci source in your $GOPATH
$ mkdir -p $GOPATH/src/github.com/circleci
$ cd $GOPATH/src/github.com/circleci

# Clone the repo
$ git clone git@github.com/circleci/circleci-cli
$ cd circleci-cli
```

### 2. Build the binary

```
$ make
```

### 3. Run Diagnostic check

```
$ ./build/target/darwin/amd64/circleci-cli diagnostic

Please enter your CircleCI API token:
OK.
Your configuration has been created in `/home/zzak/.circleci/cli.yml`.
It can edited manually for advanced settings.

---
CircleCI CLI Diagnostics
---

Config found: `/home/zzak/.circleci/cli.yml`
Host is: https://circleci.com
OK, got a token.
```

## Running a query

After you've setup the CLI, you can try executing a GraphQL query against the client.

Given we've written the following query in a file called `query.gql`:

``` graphql
query IntrospectionQuery {
	__schema {
		queryType { name }
		mutationType { name }
		subscriptionType { name }
		types {
			...FullType
		}
		directives {
			name
			description
		}
	}
}

fragment FullType on __Type {
	kind
	name
	description
	fields(includeDeprecated: true) {
		name
	}
}
```

You can now pipe that file to the `query` command to send it.

```
$ cat query.gql | ./build/target/darwin/amd64/circleci-cli query
```

This should pretty-print back a JSON response from the server:

```
{
	"__schema": {
		# ...Tons O' Schema
	}
}
```
