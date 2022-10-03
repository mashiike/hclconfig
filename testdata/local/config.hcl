locals {
    addr = "http://127.0.0.1"
    hoge_port = 8080
    tora_port = local.hoge_port + 1
}

version = "2"
io_mode = "readonly"

service "http" "hoge" {
  addr = local.addr
  port = local.hoge_port
}

service "http" "tora" {
  addr = local.addr
  port = local.tora_port
}

