//
// Host monitor agent, Sean Caron, scaron@umich.edu
//

//
// format of data transmission to server:
//  timestamp:hostname:num_cpus:memtotal:onemin:fivemin:fifteenmin:swap_pct:disk1,disk1_pct,disk2,disk2_pct,...
//

package main

import (
    "bufio"
    "fmt"
    "os"
    "os/exec"
    "strings"
    "strconv"
    "time"
    "net"
    "log"
)

func main() {

    var swap_used_pct float64

    if ((len(os.Args) != 3) || (os.Args[1] != "-h")) {
        log.Fatalf("Usage: %s -h server\n", os.Args[0])
    }

    nc := getNumCPUs()
    
    om, fivm, fifm := getLoadAvgs()
    
    mt, _, st, sf := getMemInfo()
    swap_used_pct = ((float64(st)-float64(sf))/float64(st))*100.0
    
    diskReport := getDiskInfo()
    
    timestamp := time.Now().Unix()
    
    host, _ := os.Hostname()
    
    if (strings.Index(host, ".") != -1) {
        host = host[0:strings.Index(host, ".")]
    }
        
    conn, err := net.Dial("tcp", os.Args[2]+":5962")
    if err != nil {
        log.Fatalf("Error calling net.Dial()")
    }
    
    fmt.Fprintf(conn, "%d,%s,%d,%d,%f,%f,%f,%f,%s\n", timestamp, host, nc, mt, om, fivm, fifm, swap_used_pct, diskReport)
    
    conn.Close()
}

//
// Get number of installed CPUs
//

func getNumCPUs() int {

    var numCPUs int
    
    f,err := os.Open("/proc/cpuinfo")

    if ( err != nil ) {
        return 0
    }

    input := bufio.NewScanner(f)
    
    numCPUs = 0
    
    for input.Scan() {
        inp := input.Text();
	if (strings.Contains(inp, "processor")) {
	    numCPUs++
	}
    }
    
    f.Close()
    
    return numCPUs
}

//
// Get load averages
//

func getLoadAvgs() (float64, float64, float64) {

    var loadOneMin, loadFiveMin, loadFifteenMin float64
    
    f,err := os.Open("/proc/loadavg")
    
    if ( err != nil ) {
        return 0.0, 0.0, 0.0
    }
    
    input := bufio.NewScanner(f)
    
    input.Scan()
    
    inp := input.Text();
	
    averages := strings.Fields(inp)
    
    loadOneMin, _ = strconv.ParseFloat(averages[0], 64)
    loadFiveMin, _ = strconv.ParseFloat(averages[1], 64)
    loadFifteenMin, _ = strconv.ParseFloat(averages[2], 64)
    
    f.Close()
    
    return loadOneMin, loadFiveMin, loadFifteenMin
}

//
// Get memory and swap information
//

func getMemInfo() (int64, int64, int64, int64) {

    var memTotal, memFree, swapTotal, swapFree int64
    
    f, err := os.Open("/proc/meminfo")
    
    if ( err != nil ) {
        return 0, 0, 0, 0
    }
    
    input := bufio.NewScanner(f)
    
    for input.Scan() {
        inp := input.Text()
	
	data := strings.Fields(inp)
	
	if ( data[0] == "MemTotal:" ) {
	    memTotal, _ = strconv.ParseInt(data[1], 10, 64)
	}
	
	if ( data[0] == "MemFree:" ) {
	    memFree, _ = strconv.ParseInt(data[1], 10, 64)
	}
	
	if ( data[0] == "SwapTotal:" ) {
	    swapTotal, _ = strconv.ParseInt(data[1], 10, 64)
	}
	
	if ( data[0] == "SwapFree:" ) {
	    swapFree, _ = strconv.ParseInt(data[1], 10, 64)
	}	
	
    }
    
    f.Close()
    
    return memTotal, memFree, swapTotal, swapFree   
}

//
// Get partition utilization
//

func getDiskInfo() string {

    var returned string
    
    command := "df"
    args := []string{"-k", "-l"}
    
    cmd := exec.Command(command, args...)
    
    reader, err := cmd.StdoutPipe()
    if (err != nil) {
        return ""
    }
    
    err = cmd.Start()
    if (err != nil) {
        return ""
    }
    
    scanner := bufio.NewScanner(reader)
    
    returned = ""
    
    for scanner.Scan() {
        inp := scanner.Text()
	data := strings.Fields(inp)
	//fmt.Printf("%s %s\n", data[4], data[5])
	
	data[4] = strings.Trim(data[4], "%")
	
	if ( data[5] == "/" ) {
	    //rootUsed := data[4]
	    returned = returned + data[5] + " " + data[4] + " "
	}
	
	if ( data[5] == "/exports" ) {
	    //exportsUsed := data[4]
	    returned = returned + data[5] + " " + data[4] + " "
	}
	
	if ( data[5] == "/incoming" ) {
	    //incomingUsed := data[4]
	    returned = returned + data[5] + " " + data[4] + " "
	}
	
	if ( data[5] == "/working" ) {
	    //workingUsed := data[4]
	    returned = returned + data[5] + " " + data[4] + " "
	}
	
	if ( data[5] == "/home" ) {
	    //homeUsed := data[4]
	    returned = returned + data[5] + " " + data[4] + " "
	}
	
	if ( data[5] == "/var" ) {
	    //varUsed := data[4]
	    returned = returned + data[5] + " " + data[4] + " "
	}
	
	if ( data[5] == "/tmp" ) {
	    //tmpUsed := data[4]
	    returned = returned + data[5] + " " + data[4] + " "
	}
    }
    
    err = cmd.Wait()
    if (err != nil) {
        return ""
    }

    returned = strings.Trim(returned, " ")
    
    return returned
}
