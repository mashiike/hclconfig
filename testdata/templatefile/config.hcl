io_mode = "readwrite"

locals {
    env = "prod"
}

service "http" "hoge" {
  addr = trimspace(templatefile("template/addr.hcl", {env = local.env}))
  port = 8080
}
