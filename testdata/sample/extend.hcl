service "http" "piyo" {
  addr = "http://127.0.0.1"
  port = service.http.tora.port + 1
}

service "http" "fuga" {
  addr = "http://127.0.0.1"
  port = parseint(must_env("PORT"), 10) + 4
}
