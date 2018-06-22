# circleci-cli

This project is the seed for CircleCI's new command-line application.

## Getting Started


### 1. Get the latest binary

Download the [latest release](https://github.com/CircleCI-Public/circleci-cli/releases/latest) from GitHub for your operating system. If you're on a Mac, this would be `circleci-cli_0.1.X_darwin_amd64.tar.gz`.

### 2. Put the binary in your $PATH

```
$ tar -xvzf circleci-cli_0.1.X_darwin_amd64.tar.gz
$ mv circleci-beta /usr/local/bin
```

### 3. Run a Diagnostic check

```
$ circleci-beta diagnostic

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
$ cat query.gql | circleci-beta query
```

This should pretty-print back a JSON response from the server:

```
{
	"__schema": {
		# ...Tons O' Schema
	}
}
```
