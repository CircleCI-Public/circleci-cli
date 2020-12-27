
# Aruba reference
# https://gist.github.com/bdunn313/4199906

Feature: Root Commands
  Scenario: Help text
    When I run `circleci help`
    Then the output should contain:
    """
    Use CircleCI from the command line.

    This project is the seed for CircleCI's new command-line application.

    For more help, see the documentation here: https://circleci.com/docs/2.0/local-cli/
    """
    And the exit status should be 0

  @mocked_home_directory
  Scenario: Help test with a custom host
    Given a file named ".circleci/cli.yml" with "host: foo.bar"
    When I run `circleci help`
    Then the output should not contain:
    """
    For more help, see the documentation here: https://circleci.com/docs/2.0/local-cli/
    """
    And the exit status should be 0

  @mocked_home_directory
  Scenario: The current user's token is not shown in the help test as the default value
    Given a file named ".circleci/cli.yml" with "token: jentacular"
    When I run `circleci help`
    Then the output should not contain "jentacular"
    And the exit status should be 0
