package org

policy_name["meta_policy_test"]
enable_rule["enabled"] { data.meta.vcs.branch == "main" }
enable_rule["disabled"] { data.meta.project_id != "test-project-id" }
