package org
enable_rule["branch_is_main"]
name_is_bob = "branch must be main!" { input.branch != "main" }