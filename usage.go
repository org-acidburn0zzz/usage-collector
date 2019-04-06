package main

import (
    "encoding/json"
    "log"
    "net/http"
)

// Global vars
var SDIR="/var/db/ix-stats"
var DAILYFILE string

// Submission JSON structs
//////////////////////////////////////////////////////////
type s_plugins struct {
    Name string
    Version string
}

type s_pools struct {
    Name string
    Type string
    Vdevs int
    Disks int
    Capacity int
}

type s_hw struct {
    Cpus int
    Memory int
    Nics int
}

type s_services struct {
    Name string
    Enabled bool
}

type submission_json struct {
    Platform string
    Version string
    Plugins []s_plugins `json:"plugins"`
    Pools []s_plugins `json:"pools"`
    Hardware s_hw `json:"hardware"`
    Services []s_services `json:"services"`
}

//////////////////////////////////////////////////////////

func load_daily_file() {

}

func submit(rw http.ResponseWriter, req *http.Request) {
    decoder := json.NewDecoder(req.Body)
    var s submission_json
    err := decoder.Decode(&s)
    if err != nil {
	log.Println(err)
        return
    }
    log.Println(s.Platform)
    log.Println(s.Version)
    log.Println(s.Plugins)
    log.Println(s.Pools)
    log.Println(s.Hardware)
    log.Println(s.Services)
}

func main() {
    http.HandleFunc("/submit", submit)
    log.Fatal(http.ListenAndServe(":8082", nil))
}
