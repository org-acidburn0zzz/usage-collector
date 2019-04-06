package main

import (
    "encoding/json"
    "log"
    "net/http"
    "sync"
    "time"
)

// Global vars
var SDIR="/var/db/ix-stats"

// What file to store current stats in
var DAILYFILE string

// Create our mutex we use to prevent race conditions when updating
// counters
var wlock sync.Mutex

//////////////////////////////////////////////////////////
// Tracking JSON Structs
//////////////////////////////////////////////////////////

type t_plat_count struct {
    Name string
    Version string
    Count int
}

type t_plugin_count struct {
    Name string
    Version string
    Count int
}

type t_pool_vdev_count struct {
    Type string
    Vdevs int
}

type t_pool_disk_count struct {
    Type string
    Disks int
}

type t_pool_capacity_count struct {
    Type string
    Cap int
}

type tracking_json struct {
    // Store Platform Version number count
    Platforms []t_plat_count `json:"platforms"`

    // Store plugin version/number counters
    Plugins []t_plugin_count `json:"plugins"`

    // Store vdev counters for pools
    Poolvdevs []t_pool_vdev_count `json:"poolvdevs"`

    // Store counter of pool disk numbers
    Pooldisks []t_pool_disk_count `json:"pooldisks"`

    // Store the total capacity of globally managed storage
    Poolcapacity []t_pool_capacity_count `json:"poolcapacity"`
}

//////////////////////////////////////////////////////////
// Submission JSON structs
//////////////////////////////////////////////////////////
type s_plugins struct {
    Name string
    Version string
}

type s_pools struct {
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

// Clear out the JSON structure counters
func zero_out_stats() {
	wlock.Lock()

	wlock.Unlock()
}

// Get the latest daily file to store data
func get_daily_filename() {
	t := time.Now()
	oldfile := DAILYFILE
	DAILYFILE = SDIR + "/" + t.Format("20060102") + ".json"
	if ( oldfile != DAILYFILE ) {
	    // Timestamp has changed, lets reset our in-memory json counters structure
	    zero_out_stats()
	}

}

// Load the daily file into memory
func load_daily_file() {
    get_daily_filename()

}

func parse_data(s submission_json) {
    wlock.Lock()

    log.Println(s.Platform)
    log.Println(s.Version)
    log.Println(s.Plugins)
    log.Println(s.Pools)
    log.Println(s.Hardware)
    log.Println(s.Services)

    wlock.Unlock()
}

// Getting a new submission
func submit(rw http.ResponseWriter, req *http.Request) {
    decoder := json.NewDecoder(req.Body)

    // Decode the POST data json struct
    var s submission_json
    err := decoder.Decode(&s)
    if err != nil {
	log.Println(err)
        return
    }

    // Do things with the data
    parse_data(s)
}

// Lets do it!
func main() {

    // Read the daily file into memory at startup
    load_daily_file()

    // Start our HTTP listener
    http.HandleFunc("/submit", submit)
    log.Fatal(http.ListenAndServe(":8082", nil))
}
