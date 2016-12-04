//
// Host monitor agent, Sean Caron, scaron@umich.edu
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
    "encoding/json"
)

type Message struct {
    Timestamp int64
    Hostname string
    NumCPUs int64
    Memtotal int64
    LoadOne float64
    LoadFive float64
    LoadFifteen float64
    SwapUsed float64
    KernelVer string
    Release string
    Uptime string
    DiskReport string
}


func main() {
    var m Message

    if ((len(os.Args) != 3) || (os.Args[1] != "-h")) {
        log.Fatalf("Usage: %s -h server\n", os.Args[0])
    }

    m.NumCPUs = getNumCPUs()

    m.LoadOne, m.LoadFive, m.LoadFifteen = getLoadAvgs()

    m.KernelVer = getKernelVer()
    m.Release = getRelease()

    m.Uptime = getUptime()

    mt, _, st, sf := getMemInfo()
    m.Memtotal = mt
    m.SwapUsed = ((float64(st)-float64(sf))/float64(st))*100.0

    m.DiskReport = getDiskInfo()

    m.Timestamp = time.Now().Unix()

    m.Hostname, _ = os.Hostname()

    if (strings.Index(m.Hostname, ".") != -1) {
        m.Hostname = m.Hostname[0:strings.Index(m.Hostname, ".")]
    }

    conn, err := net.Dial("tcp", os.Args[2]+":5962")
    if err != nil {
        log.Fatalf("Error calling net.Dial()")
    }

    rpt, err := json.Marshal(m)

    if (err != nil) {
        log.Fatalf("Error attempting to marshal JSON")
    }

    //fmt.Fprintf(conn, "%s\n", rpt)
    fmt.Printf("%s\n", rpt)

    conn.Close()
}

//
// Get number of installed CPUs
//

func getNumCPUs() int64 {
    var numCPUs int64

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
// Get release
//

func getRelease() (string) {
  var r string

  f, err := os.Open("/etc/redhat-release")

  // Debian and derived distributions need slightly more processing
  if (err != nil) {
    f, err = os.Open("/etc/os-release")

    // Information unavailable or this is a distro that we don't support
    if (err != nil) {
      return "unknown"
    }

    input := bufio.NewScanner(f)
    for input.Scan() {
      i := input.Text()
      d := strings.Split(i, "=")
      if (d[0] == "PRETTY_NAME") {
        r = d[1][1:len(d[1])-1]
      }
    }
  } else {
    // Red Hat and derived distributions are the easiest case
    input := bufio.NewScanner(f)
    input.Scan()
    r = input.Text()
  }

  f.Close()

  return r
}

//
// Get kernel version
//

func getKernelVer() (string) {
    f, err := os.Open("/proc/version")

    if (err != nil) {
        return "unknown"
    }

    input := bufio.NewScanner(f)
    input.Scan()
    inp := input.Text()

    vt := strings.Fields(inp)

    f.Close()

    return vt[2]
}

//
// Get uptime
//

func getUptime() (string) {
    f, err := os.Open("/proc/uptime")

    if (err != nil) {
        return "unknown"
    }

    input := bufio.NewScanner(f)
    input.Scan()
    inp := input.Text()

    u := strings.Fields(inp)

    f.Close()

    return u[0]
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

	data[4] = strings.Trim(data[4], "%")

	if ( data[5] == "/" ) {
	    returned = returned + data[5] + " " + data[4] + " "
	}

	if ( data[5] == "/exports" ) {
	    returned = returned + data[5] + " " + data[4] + " "
	}

	if ( data[5] == "/incoming" ) {
	    returned = returned + data[5] + " " + data[4] + " "
	}

	if ( data[5] == "/working" ) {
	    returned = returned + data[5] + " " + data[4] + " "
	}

	if ( data[5] == "/home" ) {
	    returned = returned + data[5] + " " + data[4] + " "
	}

        if (data[5] == "/exports/home") {
	    returned = returned + data[5] + " " + data[4] + " "
        }

	if (data[5] == "/var") {
	    returned = returned + data[5] + " " + data[4] + " "
	}

	if (data[5] == "/tmp") {
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
