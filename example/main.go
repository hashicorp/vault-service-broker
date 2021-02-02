package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

// IndexHandler returns a printout of the request it received for debugging.
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	header, _ := json.Marshal(r.Header)

	w.WriteHeader(200)
	fmt.Fprintf(w, `{
	"request": {
		"method": "%s", 
		"url": "%s", 
		"header": %s, 
		"body": "%s"
	}
}`, r.Method, r.URL, header, body)
}

func main() {
	http.HandleFunc("/", IndexHandler)

	var port string
	if port = os.Getenv("PORT"); len(port) == 0 {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
