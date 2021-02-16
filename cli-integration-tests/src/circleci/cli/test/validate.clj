(ns circleci.cli.test.validate
  (:require [circleci.cli.test.util :as util]
            [greenlight.test :refer [deftest]]
            [greenlight.step :as step :refer [defstep]]))

;; Quick intro to Greenlight:
;;
;; A "test" is a call to greenlight.test/deftest, and roughly corresponds to
;; one use case, in which many things can happen. It contains many steps which will
;; run and potentially clean themselves up in reverse order during the teardown phase.
;; If a test fails, other tests will still run.
;;
;; A "step" is a map containing various :greenlight.step/* keys, and represents
;; roughly one operation in a test scenario. It will execute some arbitrary behavior and
;; make zero or more assertions using the built-in clojure.test framework.
;; If any of those assertions fail, the test halts, no more steps are executed,
;; and the existing steps that ran so far are cleaned up in reverse order.
;;
;; Here we define our tests, which contain many steps. So far all of the steps
;; are constructed by calling helper functions in the util namespace, but we could
;; also manually construct step maps here if needed.

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
             {:exit-nonzero true
              :err-contains "A job must have one of `docker`, `machine`, `macos` or `executor`"}))
