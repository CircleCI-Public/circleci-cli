
# Aruba reference
# https://gist.github.com/bdunn313/4199906

Feature: Context integration tests

  @mocked_home_directory
  Scenario: when listing contexts without a token
    When I run `circleci context list github foo --skip-update-check --token ""`
    Then the output should contain:
    """
    Error: please set a token with 'circleci setup'
    You can create a new personal API token here:
    https://circleci.com/account/api
    """
    And the exit status should be 255
