(ns circleci.cli.test.validate
  (:require [circleci.cli.test.util :as util]
            [greenlight.test :refer [deftest]]
            [greenlight.step :as step :refer [defstep]]))

(deftest validate-good-config
  "Validate config that is valid"
  (util/add-file! "config.yml" "
jobs:
  build:
    machine: true
    steps: [checkout]"))
