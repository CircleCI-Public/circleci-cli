
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

  Scenario: Checking a valid config file with an orb
    Given a file named "config.yml" with:
    """
    version: 2.1

    orbs:
      node: circleci/node@5.0.3
  
    jobs:
      datadog-hello-world:
        docker:
          - image: cimg/base:stable
        steps:
          - run: |
              echo "doing something really cool"
    workflows:
      datadog-hello-world:
        jobs:
          - datadog-hello-world
    """
    When I run `circleci config validate --skip-update-check -c config.yml`
    Then the exit status should be 0
    And the output should contain "Config file at config.yml is valid"

  Scenario: Checking a valid config against the k9s server
    Given a file named "config.yml" with:
    """
    version: 2.1
 
    jobs:
      datadog-hello-world:
        docker:
          - image: cimg/base:stable
        steps:
          - run: |
              echo "doing something really cool"
    workflows:
      datadog-hello-world:
        jobs:
          - datadog-hello-world
    """
    When I run `circleci --host https://k9s.sphereci.com config validate --skip-update-check -c config.yml`
    Then the exit status should be 0
    And the output should contain "Config file at config.yml is valid"

  Scenario: Checking a valid config file with an orb
    Given a file named "config.yml" with:
    """
    version: 2.1

    orbs:
      node: circleci/node@5.0.3
  
    jobs:
      datadog-hello-world:
        docker:
          - image: cimg/base:stable
        steps:
          - run: |
              echo "doing something really cool"
    workflows:
      datadog-hello-world:
        jobs:
          - datadog-hello-world
    """
    When I run `circleci config validate --skip-update-check -c config.yml`
    Then the exit status should be 0
    And the output should contain "Config file at config.yml is valid"

  Scenario: Checking a valid config file with a private org
    Given a file named "config.yml" with:
    """
    version: 2.1

    orbs:
      node: circleci/node@5.0.3
  
    jobs:
      datadog-hello-world:
        docker:
          - image: cimg/base:stable
        steps:
          - run: |
              echo "doing something really cool"
    workflows:
      datadog-hello-world:
        jobs:
          - datadog-hello-world
    """
    When I run `circleci config validate --skip-update-check --org-id bb604b45-b6b0-4b81-ad80-796f15eddf87 -c config.yml`
    Then the output should contain "Config file at config.yml is valid"
    And the exit status should be 0

  Scenario: Checking a valid config file with a non-existant orb
    Given a file named "config.yml" with:
    """
    version: 2.1

    orbs:
      node: circleci/doesnt-exist@5.0.3
  
    jobs:
      datadog-hello-world:
        docker:
          - image: cimg/base:stable
        steps:
          - run: |
              echo "doing something really cool"
    workflows:
      datadog-hello-world:
        jobs:
          - datadog-hello-world
    """
    When I run `circleci config validate --skip-update-check -c config.yml`
    Then the exit status should be 255
    And the output should contain "config compilation contains errors"

  Scenario: Checking a valid config file with pipeline-parameters
    Given a file named "config.yml" with:
    """
    version: 2.1
    
    parameters:
      foo:
        type: string
        default: "bar"
  
    jobs:
      datadog-hello-world:
        docker:
          - image: cimg/base:stable
        steps:
          - run: |
              echo "doing something really cool"
              echo << pipeline.parameters.foo >>
    workflows:
      datadog-hello-world:
        jobs:
          - datadog-hello-world
    """
    When I run `circleci config process config.yml --pipeline-parameters "foo: fighters"`
    Then the output should contain "fighters"
    And the exit status should be 0

  Scenario: Testing new type casting works as expected
    Given a file named "config.yml" with:
    """
    version: 2.1

    jobs:
      datadog-hello-world:
        docker:
          - image: cimg/base:stable
        parameters:
          an-integer:
            description: a test case to ensure parameters are passed correctly
            type: integer
            default: -1
        steps:
          - unless:
              condition:
                equal: [<< parameters.an-integer >>, -1]
              steps:
                - run: echo "<< parameters.an-integer >> - test" 
    workflows:
      main-workflow:
        jobs:
        - datadog-hello-world:
            an-integer: << pipeline.number >>
    """ 
    When I run `circleci config process config.yml`
    Then the output should contain "1 - test"
    And the exit status should be 0

  Scenario: Checking a valid config file with default pipeline params
    Given a file named "config.yml" with:
    """
    version: 2.1
    
    parameters:
      foo:
        type: string
        default: "bar"
  
    jobs:
      datadog-hello-world:
        docker:
          - image: cimg/base:stable
        steps:
          - run: |
              echo "doing something really cool"
              echo << pipeline.parameters.foo >>
    workflows:
      datadog-hello-world:
        jobs:
          - datadog-hello-world
    """
    When I run `circleci config process config.yml`
    Then the output should contain "bar"
    And the exit status should be 0

  Scenario: Checking a valid config file with file pipeline-parameters
    Given a file named "config.yml" with:
    """
    version: 2.1
    
    parameters:
      foo:
        type: string
        default: "bar"
  
    jobs:
      datadog-hello-world:
        docker:
          - image: cimg/base:stable
        steps:
          - run: |
              echo "doing something really cool"
              echo << pipeline.parameters.foo >>
    workflows:
      datadog-hello-world:
        jobs:
          - datadog-hello-world
    """
    And I write to "params.yml" with:
    """
    foo: "totallyawesome"
    """
    When I run `circleci config process config.yml --pipeline-parameters params.yml`
    Then the output should contain "totallyawesome"
    And the exit status should be 0


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
