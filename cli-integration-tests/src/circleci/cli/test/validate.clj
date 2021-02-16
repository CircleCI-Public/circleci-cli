(ns circleci.cli.test.validate
  (:require [circleci.cli.test.util :as util]
            [greenlight.test :refer [deftest]]
            [greenlight.step :as step :refer [defstep]]))

(deftest validate-good-config
  "Validating good config says it is valid"
  (util/add-file! "config.yml" "
version: 2.1
jobs:
  build:
    machine: true
    steps: [checkout]")
  (util/cli! ["config" "validate" "-c" "config.yml"]
             {:out-contains "Config file at config.yml is valid."}))


(deftest validate-bad-config
  "Validating bad config says it is not valid"
  (util/add-file! "config.yml" "
version: 2.1
jobs:
  build:
    steps: [checkout]")
  (util/cli! ["config" "validate" "-c" "config.yml"]
             {:exit-nonzero 1
              :err-contains "A job must have one of `docker`, `machine`, `macos` or `executor`"}))
