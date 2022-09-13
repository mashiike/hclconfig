version = "1"
io_mode = "readonly"

service "http" "hoge" {
  addr = env("ADDR", "http://127.0.0.1")
  port = parseint(env("PORT", "8080"), 10)
}

service "http" "tora" {
  addr = env("ADDR", "http://127.0.0.1")
  port = service.http.hoge.port + 1
}

