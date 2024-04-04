package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// Usage: go run app/main.go -port 8081
// Example CURL: curl -d '{"game":"Mobile Legends", "gamerID":"GYUTDTE", "points":20}' -H "Content-Type: application/json" -X POST http://localhost:8081/echo

// handleEcho simply echo back the JSON body it received
func handleEcho(w http.ResponseWriter, r *http.Request) {
	// log.Printf("Sleep for 500 us\n")
	// time.Sleep(500 * time.Microsecond)

	contentType := r.Header.Get("Content-type")
	if contentType != "application/json" {
		log.Printf("invalid conntent type: %s\n", contentType)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	raw, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%s", raw)
	log.Printf("Handle request: %s%s\n", r.Host, r.RequestURI)

}

func main() {
	var flagPort int
	flag.IntVar(&flagPort, "port", 8081, "port to listen (default:8081)")
	flag.Parse()

	// start mux server and serve the JSON echo back API (/echo)
	r := mux.NewRouter()
	r.HandleFunc("/echo", handleEcho).Methods("POST")

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", flagPort),
		Handler: r,
	}
	log.Printf("listen on: %s\n", srv.Addr)
	srv.ListenAndServe()
}
