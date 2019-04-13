package main

import (
    "encoding/json"
    "log"
    "io/ioutil"
    "net/http"
    "os"
    "os/signal"
    "reflect"
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
    Count uint
}

type t_service_count struct {
    Name string
    Count uint
}

type t_service_share_count struct {
    Name string
    Count uint
}

type t_plugin_count struct {
    Name string
    Version string
    Count uint
}

type t_pool_vdev_count struct {
    Vdevs uint
    Count uint
}

type t_pool_disk_count struct {
    Disks uint
    Count uint
}

type t_pool_type_count struct {
    Type string
    Count uint
}

type t_hw_cpus_count struct {
    CPUs uint
    Count uint
}

type t_hw_memory_count struct {
    Memory uint
    Count uint
}

type t_hw_nics_count struct {
    Nics uint
    Count uint
}

type t_jails_count struct {
    Release string `json:"release"`
    Nat uint `json:"nat"`
    Vnet uint `json:"vnet"`
    Count uint `json:"count"`
}

type t_net_bridges_members_count struct {
    Members []string
    Mtu uint
    Count uint
}

type t_net_laggs_members_count struct {
    Members []string
    Type string
    Mtu uint
    Count uint
}

type t_net_phys_count struct {
    Name string
    Mtu uint
    Count uint
}

type t_net_vlans_count struct {
    Parent string
    Mtu uint
    Count uint
}

type t_networking_count struct {
    Bridges []t_net_bridges_members_count `json:"bridges"`
    Laggs []t_net_laggs_members_count `json:"laggs"`
    Phys []t_net_phys_count `json:"phys"`
    Vlans []t_net_vlans_count `json:"vlans"`
}

type t_sys_users_count struct {
    Localusers uint
    Count uint
}

type t_sys_datasets_count struct {
    Datasets uint
    Count uint
}

type t_sys_snapshots_count struct {
    Snapshots uint
    Count uint
}

type t_sys_zvols_count struct {
    Zvols uint
    Count uint
}

type t_sys struct {
    Localusers []t_sys_users_count `json:"localusers"`
    Datasets []t_sys_datasets_count `json:"datasets"`
    Snapshots []t_sys_snapshots_count `json:"snapshots"`
    Zvols []t_sys_zvols_count `json:"zvols"`
}

type tracking_json struct {
    // System Tracker
    System t_sys `json:"system"`

    // Store networking counters
    Network t_networking_count `json:"networking"`

    // Store HW counters
    CPUs []t_hw_cpus_count `json:"cpus"`
    Memory []t_hw_memory_count `json:"memory"`
    Nics []t_hw_nics_count `json:"nics"`

    // Store Jail counters
    Jails []t_jails_count `json:"jails"`

    // Store Platform Version number count
    Platforms []t_plat_count `json:"platforms"`

    // Store enabled services count
    Services []t_service_count `json:"services"`

    // Store service share counts
    ServiceShares []t_service_share_count `json:"serviceshares"`

    // Store plugin version/number counters
    Plugins []t_plugin_count `json:"plugins"`

    // Store vdev counters for pools
    PoolVdevs []t_pool_vdev_count `json:"poolvdevs"`

    // Store counter of pool disk numbers
    PoolDisks []t_pool_disk_count `json:"pooldisks"`

    // Counter for types of pools
    PoolTypes []t_pool_type_count `json:"pooltype"`

    // Counter for number of pools with encryption
    PoolEncryption uint `json:"poolencryption"`

    // Counter for number of pools with dedicated l2arc
    PoolL2arc uint `json:"pooll2arc"`

    // Counter for number of pools with dedicated zil
    PoolZil uint `json:"poolzil"`

    // Store the total capacity of globally managed storage
    PoolCapacity uint `json:"poolcapacity"`

    // Store the total used of globally managed storage
    PoolUsed uint `json:"poolused"`

    // Total number of system submissions
    SystemCount uint
}

var TJSON tracking_json

//////////////////////////////////////////////////////////
// Submission JSON structs
//////////////////////////////////////////////////////////
type s_plugins struct {
    Name string
    Version string
    Count uint
}

type s_pools struct {
    Type string
    Vdevs uint
    Disks uint
    Capacity uint
    Used uint
    Encryption bool
    Zil bool
    L2arc bool
}

type s_hw struct {
    CPUs uint
    Memory uint
    Nics uint
}

type s_services struct {
    Name string `json:"name"`
    Enabled bool `json:"enabled"`
}

type s_shares struct {
    Type string `json:"type"`
    AllowGuest bool `json:"allowguest"`
}

type s_jails struct {
    Nat bool `json:"nat"`
    Release string `json:"release"`
    Vnet bool `json:"vnet"`
}

type s_network_bridges struct {
    Count uint `json:"Count"`
    Members []string `json:"members"`
    Mtu uint `json:"mtu"`
}

type s_network_laggs struct {
    Count uint `json:"Count"`
    Members []string `json:"members"`
    Mtu uint `json:"mtu"`
    Type string `json:"type"`
}

type s_network_phys struct {
    Count uint `json:"Count"`
    Mtu uint `json:"mtu"`
    Name string `json:"name"`
}

type s_network_vlans struct {
    Count uint `json:"Count"`
    Mtu uint `json:"mtu"`
    Parent string `json:"parent"`
}

type s_network struct {
    Bridges []s_network_bridges `json:"bridges"`
    Laggs []s_network_laggs `json:"laggs"`
    Phys []s_network_phys `json:"phys"`
    Vlans []s_network_vlans `json:"vlans"`
}

type s_system struct {
    Datasets uint `json:"datasets"`
    Localusers uint `json:"localusers"`
    Snapshots uint `json:"snapshots"`
    Zvols uint `json:"zvols"`
}

type submission_json struct {
    Platform string
    Version string
    Network s_network `json:"network"`
    Jails []s_jails `json:"jails"`
    Plugins []s_plugins `json:"plugins"`
    Pools []s_pools `json:"pools"`
    Hardware s_hw `json:"hardware"`
    Services []s_services `json:"services"`
    Shares []s_shares `json:"shares"`
    System s_system `json:"system"`
}

//////////////////////////////////////////////////////////
// Parsing and counting data routines below
//////////////////////////////////////////////////////////

func parse_data(s submission_json) {

    // Do this all within a locked mutex
    wlock.Lock()

    // Check if the daily file needs to roll over
    get_daily_filename()

    // Increase total number of systems
    TJSON.SystemCount++

    // Update our in-memory counters
    increment_platform(s)
    increment_services(s)
    increment_service_shares(s)
    increment_pool_vdev(s)
    increment_pool_disks(s)
    increment_pool_types(s)
    increment_pool_encryption(s)
    increment_pool_zil(s)
    increment_pool_l2arc(s)
    increment_pool_capacity(s)
    increment_pool_used(s)

    increment_cpus(s)
    increment_memory(s)
    increment_nics(s)

    increment_jails(s)
    increment_plugins(s)

    increment_net_bridges(s)
    increment_net_vlans(s)
    increment_net_laggs(s)
    increment_net_phys(s)

    increment_sys_users(s)
    increment_sys_zvols(s)
    increment_sys_snapshots(s)
    increment_sys_datasets(s)

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

func increment_sys_snapshots(s submission_json) {
    var snapcount uint
    // Snapshots vary wildly, lets get some rough approx
    if ( s.System.Snapshots > 10000 ) {
        snapcount = 10000
    } else if (s.System.Snapshots > 5000 ) {
        snapcount = 5000
    } else if (s.System.Snapshots > 1000 ) {
        snapcount = 5000
    } else if (s.System.Snapshots > 500 ) {
        snapcount = 500
    } else if (s.System.Snapshots > 100 ) {
        snapcount = 100
    } else if (s.System.Snapshots > 50 ) {
        snapcount = 50
    } else if (s.System.Snapshots > 25 ) {
        snapcount = 25
    } else {
        snapcount = 10
    }

    for i, _ := range TJSON.System.Snapshots {
	if ( TJSON.System.Snapshots[i].Snapshots == snapcount) {
		TJSON.System.Snapshots[i].Count++
		return
	}
    }
    var newEntry t_sys_snapshots_count
    newEntry.Snapshots = snapcount
    newEntry.Count = 1
    TJSON.System.Snapshots = append(TJSON.System.Snapshots, newEntry)
}

func increment_sys_zvols(s submission_json) {
    for i, _ := range TJSON.System.Zvols {
	if ( TJSON.System.Zvols[i].Zvols == s.System.Zvols) {
		TJSON.System.Zvols[i].Count++
		return
	}
    }
    var newEntry t_sys_zvols_count
    newEntry.Zvols = s.System.Zvols
    newEntry.Count = 1
    TJSON.System.Zvols = append(TJSON.System.Zvols, newEntry)
}

func increment_sys_datasets(s submission_json) {
    for i, _ := range TJSON.System.Datasets {
	if ( TJSON.System.Datasets[i].Datasets == s.System.Datasets) {
		TJSON.System.Datasets[i].Count++
		return
	}
    }
    var newEntry t_sys_datasets_count
    newEntry.Datasets = s.System.Datasets
    newEntry.Count = 1
    TJSON.System.Datasets = append(TJSON.System.Datasets, newEntry)
}

func increment_sys_users(s submission_json) {
    for i, _ := range TJSON.System.Localusers {
	if ( TJSON.System.Localusers[i].Localusers == s.System.Localusers) {
		TJSON.System.Localusers[i].Count++
		return
	}
    }
    var newEntry t_sys_users_count
    newEntry.Localusers = s.System.Localusers
    newEntry.Count = 1
    TJSON.System.Localusers = append(TJSON.System.Localusers, newEntry)
}

func increment_plugins(s submission_json) {
    var found bool
    for j, _ := range s.Plugins {
	found = false
        for i, _ := range TJSON.Plugins {
	    if ( TJSON.Plugins[i].Name == s.Plugins[j].Name && TJSON.Plugins[i].Version == s.Plugins[j].Version ) {
                TJSON.Plugins[i].Count++
		found = true
                break
             }
         }

        if ( found ) {
		continue
        }

        var newEntry t_plugin_count
        newEntry.Name = s.Plugins[j].Name
        newEntry.Version = s.Plugins[j].Version
        newEntry.Count = 1
        TJSON.Plugins = append(TJSON.Plugins, newEntry)
    }
}

func increment_net_vlans(s submission_json) {
    var found bool
    for j, _ := range s.Network.Vlans {
	found = false
        for i, _ := range TJSON.Network.Vlans {
	    if ( s.Network.Vlans[j].Mtu == TJSON.Network.Vlans[i].Mtu ) && ( s.Network.Vlans[j].Parent == TJSON.Network.Vlans[i].Parent ) {
                TJSON.Network.Vlans[i].Count++
		found = true
                break
             }
         }

        if ( found ) {
		continue
        }

        var newEntry t_net_vlans_count
        newEntry.Mtu = s.Network.Vlans[j].Mtu
        newEntry.Parent = s.Network.Vlans[j].Parent
        newEntry.Count = 1
        TJSON.Network.Vlans = append(TJSON.Network.Vlans, newEntry)
    }
}

func increment_net_phys(s submission_json) {
    var found bool
    for j, _ := range s.Network.Phys {
	found = false
        for i, _ := range TJSON.Network.Phys {
	    if ( s.Network.Phys[j].Mtu == TJSON.Network.Phys[i].Mtu ) && ( s.Network.Phys[j].Name == TJSON.Network.Phys[i].Name ) {
                TJSON.Network.Phys[i].Count++
		found = true
                break
             }
         }

        if ( found ) {
		continue
        }

        var newEntry t_net_phys_count
        newEntry.Mtu = s.Network.Phys[j].Mtu
        newEntry.Name = s.Network.Phys[j].Name
        newEntry.Count = 1
        TJSON.Network.Phys = append(TJSON.Network.Phys, newEntry)
    }
}

func increment_net_laggs(s submission_json) {
    var found bool
    for j, _ := range s.Network.Laggs {
	found = false
        for i, _ := range TJSON.Network.Laggs {
	    if ( reflect.DeepEqual( TJSON.Network.Laggs[i].Members, s.Network.Laggs[j].Members ) && (s.Network.Laggs[j].Mtu == TJSON.Network.Laggs[i].Mtu ) && ( s.Network.Laggs[j].Type == TJSON.Network.Laggs[i].Type ) ) {
                TJSON.Network.Laggs[i].Count++
		found = true
                break
             }
         }

        if ( found ) {
		continue
        }

        var newEntry t_net_laggs_members_count
        newEntry.Members = s.Network.Laggs[j].Members
        newEntry.Mtu = s.Network.Laggs[j].Mtu
        newEntry.Type = s.Network.Laggs[j].Type
        newEntry.Count = 1
        TJSON.Network.Laggs = append(TJSON.Network.Laggs, newEntry)
    }
}

func increment_net_bridges(s submission_json) {
    var found bool
    for j, _ := range s.Network.Bridges {
	found = false
        for i, _ := range TJSON.Network.Bridges {
	    if ( reflect.DeepEqual(TJSON.Network.Bridges[i].Members, s.Network.Bridges[j].Members) && s.Network.Bridges[j].Mtu == TJSON.Network.Bridges[i].Mtu  ) {
                TJSON.Network.Bridges[i].Count++
		found = true
                break
             }
         }

        if ( found ) {
		continue
        }

        var newEntry t_net_bridges_members_count
        newEntry.Members = s.Network.Bridges[j].Members
        newEntry.Mtu = s.Network.Bridges[j].Mtu
        newEntry.Count = 1
        TJSON.Network.Bridges = append(TJSON.Network.Bridges, newEntry)
    }
}

func increment_jails(s submission_json) {
    var found bool
    for j, _ := range s.Jails {
	found = false
        for i, _ := range TJSON.Jails {
	    if ( TJSON.Jails[i].Release == s.Jails[j].Release ) {
                TJSON.Jails[i].Count++
		if ( s.Jails[j].Vnet ) {
		    TJSON.Jails[i].Vnet++
		}
		if ( s.Jails[j].Nat ) {
		    TJSON.Jails[i].Nat++
		}
		found = true
                break
             }
         }

        if ( found ) {
		continue
        }

        var newEntry t_jails_count
        newEntry.Release= s.Jails[j].Release
        newEntry.Count = 1
	if ( s.Jails[j].Vnet ) {
	    newEntry.Vnet = 1
	}
	if ( s.Jails[j].Nat ) {
	    newEntry.Nat = 1
	}
        TJSON.Jails = append(TJSON.Jails, newEntry)
    }
}


func increment_nics(s submission_json) {
    var found bool
    if ( TJSON.Nics == nil ) {
        var newEntry t_hw_nics_count
        newEntry.Nics = s.Hardware.Nics
        newEntry.Count = 1
        TJSON.Nics = append(TJSON.Nics, newEntry)
	return
    }
    for i, _ := range TJSON.Nics {
        found = false
        if ( TJSON.Nics[i].Nics == s.Hardware.Nics ) {
            TJSON.Nics[i].Count++
            found = true
            break
        }

        if ( found ) {
             continue
        }

        var newEntry t_hw_nics_count
        newEntry.Nics = s.Hardware.Nics
        newEntry.Count = 1
        TJSON.Nics = append(TJSON.Nics, newEntry)
    }
}

func increment_memory(s submission_json) {
    var found bool
    if ( TJSON.Memory == nil ) {
        var newEntry t_hw_memory_count
        newEntry.Memory = s.Hardware.Memory
        newEntry.Count = 1
        TJSON.Memory = append(TJSON.Memory, newEntry)
	return
    }
    for i, _ := range TJSON.Memory {
        found = false
        if ( TJSON.Memory[i].Memory == s.Hardware.Memory ) {
            TJSON.Memory[i].Count++
            found = true
            break
        }

        if ( found ) {
             continue
        }

        var newEntry t_hw_memory_count
        newEntry.Memory = s.Hardware.Memory
        newEntry.Count = 1
        TJSON.Memory = append(TJSON.Memory, newEntry)
    }
}

func increment_cpus(s submission_json) {
    var found bool
    if ( TJSON.CPUs == nil ) {
        var newEntry t_hw_cpus_count
        newEntry.CPUs = s.Hardware.CPUs
        newEntry.Count = 1
        TJSON.CPUs = append(TJSON.CPUs, newEntry)
	return
    }
    for i, _ := range TJSON.CPUs {
        found = false
        if ( TJSON.CPUs[i].CPUs == s.Hardware.CPUs ) {
            TJSON.CPUs[i].Count++
            found = true
            break
        }

        if ( found ) {
             continue
        }

        var newEntry t_hw_cpus_count
        newEntry.CPUs = s.Hardware.CPUs
        newEntry.Count = 1
        TJSON.CPUs = append(TJSON.CPUs, newEntry)
    }
}

func increment_pool_used(s submission_json) {
    for j, _ := range s.Pools {
	if ( s.Pools[j].Used > 0 ) {
	    TJSON.PoolUsed = TJSON.PoolUsed + s.Pools[j].Used
	}
    }
}

func increment_pool_capacity(s submission_json) {
    for j, _ := range s.Pools {
	if ( s.Pools[j].Capacity > 0 ) {
	    TJSON.PoolCapacity = TJSON.PoolCapacity + s.Pools[j].Capacity
	}
    }
}

func increment_pool_encryption(s submission_json) {
    for j, _ := range s.Pools {
	if ( s.Pools[j].Encryption ) {
	    TJSON.PoolEncryption++
	}
    }
}

func increment_pool_zil(s submission_json) {
    for j, _ := range s.Pools {
	if ( s.Pools[j].Zil ) {
	    TJSON.PoolZil++
	}
    }
}

func increment_pool_l2arc(s submission_json) {
    for j, _ := range s.Pools {
	if ( s.Pools[j].L2arc ) {
	    TJSON.PoolL2arc++
	}
    }
}

func increment_pool_types(s submission_json) {
    var found bool
    for j, _ := range s.Pools {
	found = false
        for i, _ := range TJSON.PoolTypes {
	    if ( TJSON.PoolTypes[i].Type == s.Pools[j].Type ) {
                TJSON.PoolTypes[i].Count++
		found = true
                break
             }
         }

        if ( found ) {
		continue
        }

        var newEntry t_pool_type_count
        newEntry.Type= s.Pools[j].Type
        newEntry.Count = 1
        TJSON.PoolTypes = append(TJSON.PoolTypes, newEntry)
    }
}

func increment_pool_disks(s submission_json) {
    var found bool
    for j, _ := range s.Pools {
	found = false
        for i, _ := range TJSON.PoolDisks {
	    if ( TJSON.PoolDisks[i].Disks == s.Pools[j].Disks ) {
                TJSON.PoolDisks[i].Count++
		found = true
                break
             }
         }

        if ( found ) {
		continue
        }

        var newEntry t_pool_disk_count
        newEntry.Disks= s.Pools[j].Disks
        newEntry.Count = 1
        TJSON.PoolDisks = append(TJSON.PoolDisks, newEntry)
    }
}

func increment_pool_vdev(s submission_json) {
    var found bool
    for j, _ := range s.Pools {
	found = false
        for i, _ := range TJSON.PoolVdevs {
	    if ( TJSON.PoolVdevs[i].Vdevs == s.Pools[j].Vdevs ) {
                TJSON.PoolVdevs[i].Count++
		found = true
                break
             }
         }

        if ( found ) {
		continue
        }

        var newEntry t_pool_vdev_count
        newEntry.Vdevs= s.Pools[j].Vdevs
        newEntry.Count = 1
        TJSON.PoolVdevs = append(TJSON.PoolVdevs, newEntry)
    }
}

func increment_platform(s submission_json) {
    for i, _ := range TJSON.Platforms {
	if ( TJSON.Platforms[i].Name == s.Platform && TJSON.Platforms[i].Version == s.Version ) {
		TJSON.Platforms[i].Count++
		return
	}
    }
    var newEntry t_plat_count
    newEntry.Name = s.Platform
    newEntry.Version = s.Version
    newEntry.Count = 1
    TJSON.Platforms = append(TJSON.Platforms, newEntry)
}

func increment_services(s submission_json) {
    var found bool
    for j, _ := range s.Services {
        found = false
        for i, _ := range TJSON.Services {
	    if ( TJSON.Services[i].Name == s.Services[j].Name) {
		found = true
	        if ( s.Services[j].Enabled ) {
                    TJSON.Services[i].Count++
		}
                break
             }
         }
	 // Found and incremented this particular service
	 if ( found || ! s.Services[j].Enabled ) {
		 continue
	 }
	 var newEntry t_service_count
         newEntry.Name = s.Services[j].Name
	 if ( ! s.Services[j].Enabled ) {
             newEntry.Count = 0
         } else {
             newEntry.Count = 1
	 }
         TJSON.Services = append(TJSON.Services, newEntry)
    }
}

func increment_service_shares(s submission_json) {
    var found bool
    for j, _ := range s.Shares {
        found = false
        for i, _ := range TJSON.ServiceShares {
            //log.Println(s.Services[j].Name + " Shares:" + strconv.Itoa(s.Services[j].Shares))
	    if ( TJSON.ServiceShares[i].Name == s.Shares[j].Type ) {
                TJSON.ServiceShares[i].Count++
		found = true
                break
             }
         }
	 // Found and incremented this particular service
	 if ( found ) {
		 continue
	 }
	 var newEntry t_service_share_count
         newEntry.Name = s.Shares[j].Type
         newEntry.Count = 1
         TJSON.ServiceShares = append(TJSON.ServiceShares, newEntry)
    }
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

// Clear out the JSON structure counters
func zero_out_stats() {
	TJSON = tracking_json{}
}

// Get the latest daily file to store data
func get_daily_filename() {
	t := time.Now()
	newfile := SDIR + "/" + t.Format("20060102") + ".json"
	if ( newfile != DAILYFILE ) {

	    // Flush previous data to disk
	    if ( DAILYFILE != "" ) {
		flush_json_to_disk()
	    }
	    // Timestamp has changed, lets reset our in-memory json counters structure
	    zero_out_stats()

	    // Set new DAILYFILE
	    DAILYFILE = newfile
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

func flush_json_to_disk() {
    file, _ := json.MarshalIndent(TJSON, "", " ")
    _ = ioutil.WriteFile(DAILYFILE, file, 0644)
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
