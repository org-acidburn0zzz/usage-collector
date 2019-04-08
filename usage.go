package main

import (
    "encoding/json"
    "log"
    "io/ioutil"
    "net/http"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"
)

// Global vars
var SDIR="/var/db/ix-stats"

// What file to store current stats in
var DAILYFILE string

// Create our mutex we use to prevent race conditions when updating
// counters
var wlock sync.Mutex

// Counter for number of increments before a write
var WCOUNTER = 0

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
    Count int
}

type t_pool_disk_count struct {
    Type string
    Disks int
    Count int
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

    // Total number of system submissions
    SystemCount int
}

var TJSON tracking_json

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
	TJSON = tracking_json{}
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

    // No file yet? Lets clear out the struct
    if _, err := os.Stat(DAILYFILE) ; os.IsNotExist(err) {
	zero_out_stats()
        return
    }

    // Load the file into memory
    dat, err := ioutil.ReadFile(DAILYFILE)
    if ( err != nil ) {
	log.Println(err)
        log.Fatal("Failed loading daily file: " + DAILYFILE )
    }
    if err := json.Unmarshal(dat, &TJSON); err != nil {
	log.Println(err)
        log.Fatal("Failed unmarshal of JSON in DAILYFILE:")
    }
}

func increment_platform(s submission_json) {
    for i, _ := range TJSON.Platforms {
	if ( TJSON.Platforms[i].Name == s.Platform && TJSON.Platforms[i].Version == s.Version ) {
		TJSON.Platforms[i].Count++
		return
	}
    }
    var newPlat t_plat_count
    newPlat.Name = s.Platform
    newPlat.Version = s.Version
    newPlat.Count = 1
    TJSON.Platforms = append(TJSON.Platforms, newPlat)
}

func flush_json_to_disk() {
    file, _ := json.MarshalIndent(TJSON, "", " ")
    _ = ioutil.WriteFile(DAILYFILE, file, 0644)
}

func parse_data(s submission_json) {

    // Do this all within a locked mutex
    wlock.Lock()

    // Increase total number of systems
    TJSON.SystemCount++

    // Update our in-memory counters
    increment_platform(s)

    // TODO increment other submitted counters
    log.Println(s.Plugins)
    log.Println(s.Pools)
    log.Println(s.Hardware)
    log.Println(s.Services)

    // Every 5 updates, we update the JSON file on disk
    if ( WCOUNTER >= 5 ) {
	log.Println("Flushing to disk")
        flush_json_to_disk()
	WCOUNTER=0
    } else {
        WCOUNTER++
    }

    //log.Println(TJSON)

    // Unlock the mutex now
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

    // Capture SIGTERM and flush JSON to disk
    var gracefulStop = make(chan os.Signal)
    signal.Notify(gracefulStop, syscall.SIGTERM)
    signal.Notify(gracefulStop, syscall.SIGINT)
    go func() {
        sig := <-gracefulStop
	log.Println("%v", sig)
	log.Println("Caught Signal. Flushing JSON to disk")
	flush_json_to_disk()
        os.Exit(0)
    }()

    // Read the daily file into memory at startup
    load_daily_file()

    // Start our HTTP listener
    http.HandleFunc("/submit", submit)
    log.Fatal(http.ListenAndServe(":8082", nil))
}
