(ns circleci.cli.test.util
  (:require [clojure.java.io :as io]
            [clojure.java.shell :refer [sh]]
            [clojure.string :as str]
            [clojure.test :refer :all]
            [greenlight.step :as step :refer [defstep]]))

(defn add-file!
  "Reusable step that adds a file to the filesystem, then cleans it up afterwards."
  [filename content]
  #::step
  {:title (str "Create " filename)
   :name 'add-file!
   :test (fn [_]
           (step/register-cleanup! :local/file filename)
           (io/make-parents filename)
           (spit filename content))})

(defmethod step/clean! :local/file
  [_ _ filename]
  (try
    (io/delete-file filename)
    (catch Exception e
      (printf "WARN: couldn't delete %s: %s\n" filename e))))

(defn shell!
  "Reusable step that runs a shell command and validates its output."
  [args
   {:keys [exit exit-nonzero out-contains err-contains]}]
  #::step
  {:title (str/join " " args)
   :name 'shell!
   :test (fn [_]
           (let [result (apply sh args)
                 out (String. (:out result))
                 err (:err result)
                 exited-as-expected?
                 (if exit-nonzero
                   (is (not= 0 (:exit result)))
                   (is (= (or exit 0) (:exit result))))]
             (when-not exited-as-expected?
               (println "EXITED WITH" (:exit result))
               (println "STDOUT:")
               (println out)
               (println "--------")
               (println "STDERR:")
               (println err)
               (println "--------"))
             (when out-contains
               (is (str/includes? out out-contains)))
             (when err-contains
               (is (str/includes? err err-contains)))))})

(defn cli!
  "Reusable step that runs a CLI command and validates its output."
  [args verify-opts]
  ;; Add --skip-update-check because we don't need it in CI
  ;; and the update spinner adds a bunch of spam to the output
  (shell! (into ["circleci" "--skip-update-check"] args)
          verify-opts))
