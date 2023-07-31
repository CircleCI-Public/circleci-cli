package org

import future.keywords

policy_name["example_compiled"]

enable_hard["enforce_small_jobs"]

enforce_small_jobs[reason] {
	some job_name, job in input._compiled_.jobs
	job.resource_class != "small"
	reason = sprintf("job %s: resource_class must be small", [job_name])
}
