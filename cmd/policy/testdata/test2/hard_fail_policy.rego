package org

policy_name["hard_fail_test"]
enable_rule["always_hard_fails"]
hard_fail["always_hard_fails"]
always_hard_fails = "0 is not equals 1" { 0 != 1 }
