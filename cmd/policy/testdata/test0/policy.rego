package org
enable_rule["branch_is_main"]
branch_is_main = "branch must be main!" { input.branch != "main" }