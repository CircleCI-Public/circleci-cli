Feature: Orb pack
  @mocked_home_directory
  Scenario: Basic orb pack
    Given a file named "src/@orb.yml" with:
    """
    commands:
        test:
          steps:
            - run:
                command: <<include(script.sh)>>
    """
    Given a file named "src/script.sh" with "echo Hello, world!"
    When I run `circleci orb pack src`
    Then the output should contain:
    """
    commands:
        test:
            steps:
                - run:
                    command: echo Hello, world!
    """
    And the exit status should be 0

  @mocked_home_directory
  Scenario: Orb pack with multiple includes fails
    Given a file named "src/@orb.yml" with:
    """
    commands:
        test:
          steps:
            - run:
                command: <<include(script.sh)>> <<include(script.sh)>>
    """
    Given a file named "src/script.sh" with "echo Hello, world!"
    When I run `circleci orb pack src`
    Then the output should contain:
    """
    Error: An unexpected error occurred: multiple include statements: '<<include(script.sh)>> <<include(script.sh)>>'
    """
    And the exit status should be 255

  @mocked_home_directory
  Scenario: Orb pack with include statement in bigger string
    Given a file named "src/@orb.yml" with:
    """
    commands:
        test:
          steps:
            - run:
                command: include <<include(script.sh)>>
    """
    Given a file named "src/script.sh" with "echo Hello, world!"
    When I run `circleci orb pack src`
    Then the output should contain:
    """
    Error: An unexpected error occurred: entire string must be include statement: 'include <<include(script.sh)>>'
    """
    And the exit status should be 255

  @mocked_home_directory
  Scenario: Missing @orb.yml for orb packing
    When I run `circleci orb pack src`
    Then the output should contain:
    """
    Error: @orb.yml file not found, are you sure this is the Orb root?
    """
    And the exit status should be 255

  @mocked_home_directory
  Scenario: Missing script for inclusion
    Given a file named "src/@orb.yml" with:
    """
    commands:
        test:
          steps:
            - run:
                command: <<include(script.sh)>>
    """
    When I run `circleci orb pack src`
    Then the output should contain:
    """
    Error: An unexpected error occurred: could not open src/script.sh for inclusion
    """
    And the exit status should be 255
