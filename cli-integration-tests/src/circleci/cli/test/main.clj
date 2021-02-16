(ns circleci.cli.test.main
  (:require [circleci.cli.test.validate]
            [greenlight.runner :as runner]))

(defn -main
  "Main entrypoint to run all of the integration tests."
  [& args]
  (runner/run-tests!
   ;; Component system, which currently contains nothing
   (constantly {})
   ;; All the tests we want to run
   (runner/find-tests)
   ;; Options, like parallelism
   {})
  (shutdown-agents))
