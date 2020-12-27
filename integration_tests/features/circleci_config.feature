
# Aruba reference
# https://gist.github.com/bdunn313/4199906

Feature: Config checking
  Scenario: Checking a valid config file
    Given a file named "config.yml" with:
    """
    jobs:
      build:
        machine: true
        steps: [checkout]
    """
    When I run `circleci config validate --skip-update-check -c config.yml`
    Then the exit status should be 0
    And the output should contain "Config file at config.yml is valid."

  Scenario: Checking an invalid config file
    Given a file named "config.yml" with:
    """
    version: 2.1
    jobs:
      build:
        steps: [checkout]
    """
    When I run `circleci config validate --skip-update-check -c config.yml`
    Then the exit status should not be 0
    And the output should contain "A job must have one of `docker`, `machine`, `macos` or `executor`"


  Scenario: Checking a valid config file
    Given a file named "config.yml" with:
    """
    version: 2
    jobs:
      build:
        machine: true
        steps: [checkout]
    """
    When I run `circleci config process --skip-update-check config.yml`
    Then the output should match:
    """
    version: 2
    jobs:
      build:
        machine: true
        steps:
        - checkout
        environment:
        - CIRCLE_COMPARE_URL: .*
    workflows:
      version: 2
      workflow:
        jobs:
        - build
    """

  Scenario: Checking an invalid config file
    Given a file named "config.yml" with:
    """
    version: 2.1
    jobs:
      build:
        steps: [checkout]
    """
    When I run `circleci config process --skip-update-check config.yml`
    Then the output should contain "A job must have one of `docker`, `machine`, `macos` or `executor`"

  Scenario: Processing a config that has special characters
    Given I use a fixture named "config_files"
    When I run `circleci config process --skip-update-check with_percent.yml`
    Then the exit status should be 0
    And the output should contain "command: date '+%Y-%m-%dT%T%z'"

  Scenario: Running validate in a directory that is not a git repo
    When I cd to "/tmp"
    And I write to "config.yml" with:
    """
    jobs:
      build:
        machine: true
        steps: [checkout]
    """
    Then I run `git status`
    And the output should contain "fatal: not a git repository (or any of the parent directories): .git"
    And I run `circleci config validate --skip-update-check -c config.yml`
    And the output should contain "Config file at config.yml is valid."
    And the exit status should be 0
