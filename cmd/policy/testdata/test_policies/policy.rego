package org

policy_name["test"]

enable_rule["fail_if_not_main"]

fail_if_not_main = "branch must be main!" { data.meta.vcs.branch != "main" } 
