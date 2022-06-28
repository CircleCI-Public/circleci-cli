package org
enable_rule["type_is_person"]
hard_fail["type_is_person"]
type_is_person = "type must be person!" { input.type != "person" }