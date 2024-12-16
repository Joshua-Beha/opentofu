
terraform {
  # The "enabled" argument currently requires opting in to this experiment.
  experiments = [enabled_meta_arg]
}

variable "on" {
  type = bool
}

resource "test" "test" {
  enabled = var.on

  name = "boop"
}

output "result" {
  // This is in a 1-tuple just because OpenTofu treats a fully-null
  // root module output value as if it wasn't declared at all,
  // but we want to make sure we're actually testing the result
  // of this resource directly.
  value = [one(test.test[*].name)]
}
