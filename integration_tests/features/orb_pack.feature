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
  Scenario: Orb pack with multiple includes
    Given a file named "src/@orb.yml" with:
    """
    commands:
        test:
          steps:
            - run:
                command: <<include(script.sh)>> <<include(script2.sh)>>
    """
    Given a file named "src/script.sh" with "echo Hello,"
    Given a file named "src/script2.sh" with "world!"
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
  Scenario: Orb pack with include statement in bigger string
    Given a file named "src/@orb.yml" with:
    """
    commands:
        test:
          steps:
            - run:
                command: echo "<<include(script.sh)>>"
    """
    Given a file named "src/script.sh" with "Hello, world!"
    When I run `circleci orb pack src`
    Then the output should contain:
    """
    commands:
        test:
            steps:
                - run:
                    command: echo "Hello, world!"
    """
    And the exit status should be 0

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
