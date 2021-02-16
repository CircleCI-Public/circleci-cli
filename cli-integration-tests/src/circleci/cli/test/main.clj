(ns circleci.cli.test.main
  ;; Make sure all test namespaces are required here so the tests are compiled / registered
  (:require [circleci.cli.test.validate]
            [greenlight.runner :as runner]))

;; Quick intro to Greenlight:
;;
;; A "test" is a call to greenlight.test/deftest, and roughly represents one use
;; case or scenario, in which many things can happen. It contains many _steps_ which will
;; run in order, and potentially clean themselves up in reverse order during the
;; teardown phase. If a test fails, other tests will still run.
;;
;; A "step" represents roughly one operation / unit of work in a Greenlight test.
;; It will execute some arbitrary behavior, while potentially making one or more
;; assertions using the built-in clojure.test framework.
;; If any of a step's assertions fails or an exception is thrown, the whole test halts,
;; no more steps are executed, and the existing steps that ran so far are cleaned up.
;; A step could either be an intermediate task (provisioning a resource we'll need later)
;; or the ultimate task that tests the thing we want to test.
;;
;; Behind the scenes, a step is just a map that contains keys that start with :greenlight.step/...
;; This tells Greenlight how to run the step, some metadata about how to refer to the step when
;; printing the test results, and also how to pass context around between steps (a feature
;; we aren't taking advantage of yet).
;;
;; Greenlight steps are meant to be composable, in that you can use helper functions to create
;; steps to avoid re-iterating implementation details, especially for common operations
;; and intermediate provisioning steps. You can also construct step maps in-line during a deftest
;; if needed.
;;
;; In this codebase, the test suites live in `circleci.cli.test.*`, which use steps defined
;; either in that same namespace or in `circleci.cli.test.util`. The runner that invokes
;; Greenlight on all the different test suites lives here.

(defn -main
  "Main entrypoint to run all of the integration tests."
  [& args]
  (if (runner/run-tests!
       ;; Component system, which currently contains nothing
       (constantly {})
       ;; All the tests we want to run
       (runner/find-tests)
       ;; Options, like parallelism
       {})
    (shutdown-agents)
    (System/exit 1)))
