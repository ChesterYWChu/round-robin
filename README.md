# Round Robin Load Balancer

This service support two balancing type, simple round robin and weighted round robin.


# Steps

### Start Load Balancer Server
```bash
go run loadbalancer/main.go -port 8080 -urls http://localhost:8081,http://localhost:8082,http://localhost:8083
```

### Start 3 API server that simply echo back the JSON content
```bash
go run app/main.go -port 8081
go run app/main.go -port 8082
go run app/main.go -port 8083
```

### Send requests to the Load Balancer Server
```bash
curl -d '{"game":"Mobile Legends", "gamerID":"GYUTDTE", "points":20}' -H "Content-Type: application/json" -X POST http://localhost:8080/echo
```
