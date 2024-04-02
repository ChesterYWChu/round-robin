package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// Usage: go run app/main.go -port 8081
// Example CURL: curl -d '{"game":"Mobile Legends", "gamerID":"GYUTDTE", "points":20}' -H "Content-Type: application/json" -X POST http://localhost:8081/echo

func handleEcho(w http.ResponseWriter, r *http.Request) {
	time.Sleep(500000 * time.Nanosecond)

	raw, _ := io.ReadAll(r.Body)
	fmt.Fprintf(w, "post\n")
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%s\n", raw)
	log.Printf("Handle request: %s%s\n", r.Host, r.RequestURI)

}

func main() {
	var flagPort int
	flag.IntVar(&flagPort, "port", 8081, "port to listen (default:8081)")
	flag.Parse()

	r := mux.NewRouter()
	r.HandleFunc("/echo", handleEcho).Methods("POST")

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", flagPort),
		Handler: r,
	}
	log.Printf("listen on: %s\n", srv.Addr)
	srv.ListenAndServe()
}
