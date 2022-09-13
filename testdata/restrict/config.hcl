io_mode = "public"

general {
    env = ""
}

service "http" "hoge" {
  addr = "http://127.0.0.1"
  port = 8080
}

service "http" "hoge" {
  addr = "http://127.0.0.1"
  port = 8081
}
