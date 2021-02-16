(ns circleci.cli.test.util
  (:require [clojure.java.io :as io]
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
