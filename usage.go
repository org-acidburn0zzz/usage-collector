package main

import (
    "encoding/json"
    "log"
    "net/http"
)

type submission_json struct {
    Platform string
    Version string
}

func submit(rw http.ResponseWriter, req *http.Request) {
    decoder := json.NewDecoder(req.Body)
    var s submission_json
    err := decoder.Decode(&s)
    if err != nil {
        return
    }
    log.Println(s.Platform)
}

func main() {
    http.HandleFunc("/submit", submit)
    log.Fatal(http.ListenAndServe(":8082", nil))
}
