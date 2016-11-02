//
// Host monitor data collection server
//  Sean Caron scaron@umich.edu
//

package main

import (
    //"io"
    "net"
    "os"
    //"fmt"
    "strings"
    "strconv"
    "bufio"
    "math"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "net/smtp"
    "bytes"
    "log"
    "time"
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
    Uptime string
    DiskReport string
}

type Config struct {
    DBUser string
    DBPass string
    DBName string
    EMailTo string
    EMailFrom string
    LoadThresh float64
    SwapThresh float64
    LoadDThresh float64
    SwapDThresh float64
    DiskThresh int64
    DiskRInterval int64
}

//
// Configuration parameters go in global variables.
//

var g_dbUser, g_dbPass, g_dbHost, g_dbName, g_eMailTo, g_eMailFrom string
var g_loadThreshold, g_swapThreshold, g_loadFirstDThreshold, g_swapFirstDThreshold float64
var g_diskThreshold, g_diskReportInterval int64

var lastDNotify = make(map[string]int64)

func main() {
    var bindaddr, conffile string

    if (len(os.Args) != 5) {
        log.Fatalf("Usage: %s -b bindaddr -f configfile", os.Args[0])
    }

    for i := 1; i < len(os.Args); i++ {
        switch os.Args[i] {
	    case "-b":
	        bindaddr = os.Args[i+1]
            case "-f":
	        conffile = os.Args[i+1]
        }
    }

    log.Printf("Host monitor data server starting up\n")
    
    //
    // Read in the configuration file.
    //
    
    haveParam := make(map[string]bool)
    
    confFile, err := os.Open(conffile)
    
    if err != nil {
        log.Fatalf("Failed opening configuration file for reading\n")
    }
    
    inp := bufio.NewScanner(confFile)
    
    for inp.Scan() {
        line := inp.Text()

        if (len(line) > 0) {
	    theFields := strings.Fields(line)
	    key := strings.ToLower(theFields[0])
	
	    haveParam[theFields[0]] = true
	
	    switch key {
	        case "dbuser":
	            g_dbUser = theFields[1]
	        case "dbpass":
	            g_dbPass = theFields[1]
	        case "dbhost":
	            g_dbHost = theFields[1]
	        case "dbname":
	            g_dbName = theFields[1]
	        case "emailto":
	            g_eMailTo = theFields[1]
	        case "emailfrom":
	            g_eMailFrom = theFields[1]
	        case "loadthreshold":
	            g_loadThreshold, _ = strconv.ParseFloat(theFields[1], 64)
	        case "swapthreshold":
	            g_swapThreshold, _ = strconv.ParseFloat(theFields[1], 64)
	        case "loadfirstdthreshold":
	            g_loadFirstDThreshold, _ = strconv.ParseFloat(theFields[1], 64)
	        case "swapfirstdthreshold":
	            g_swapFirstDThreshold, _ = strconv.ParseFloat(theFields[1], 64)
	        case "diskthreshold":
	            g_diskThreshold, _ = strconv.ParseInt(theFields[1], 10, 64)
                case "diskreportinterval":
		    g_diskReportInterval, _ = strconv.ParseInt(theFields[1], 10, 64)
	        default:
		    log.Printf("Ignoring nonsense configuration parameter %s\n", theFields[1])
            }
	}
    }
    
    confFile.Close()
    
    //
    // Make sure no configuration directives are missing
    //
    
    if ((haveParam["dbUser"] != true) ||
        (haveParam["dbPass"] != true) ||
	(haveParam["dbHost"] != true) ||
	(haveParam["dbName"] != true) ||
        (haveParam["eMailTo"] != true) ||
	(haveParam["eMailFrom"] != true) ||
	(haveParam["loadThreshold"] != true) ||
	(haveParam["swapThreshold"] != true) ||
	(haveParam["loadFirstDThreshold"] != true) ||
	(haveParam["swapFirstDThreshold"] != true) ||
	(haveParam["diskThreshold"] != true) ||
	(haveParam["diskReportInterval"] != true)) {
	log.Fatalf("Fatal missing configuration directive\n")
    }

    log.Printf("Configuration report follows\n")
    log.Printf("  DB user: %s DB host: %s DB name: %s\n", g_dbUser, g_dbHost, g_dbName)
    log.Printf("  E-mail to: %s E-mail from: %s\n", g_eMailTo, g_eMailFrom)
    log.Printf("  Thresholds: %f %f %f %f %d\n", g_loadThreshold, g_swapThreshold, g_loadFirstDThreshold, g_swapFirstDThreshold, g_diskThreshold)
    log.Printf("  Disk report interval: %d sec\n", g_diskReportInterval)

    log.Printf("Configuration report ends\n")
    
    //
    // Start listening for connections
    //
    
    listener, err := net.Listen("tcp", bindaddr + ":5962")
    if err != nil {
	log.Fatalf("Failure calling net.Listen()\n")
    }
    
    //
    // Spin off a new Goroutine for each connection
    //
    
    for {
        conn, err := listener.Accept()
	if err != nil {
	    log.Printf("Non-zero value returned by listener.Accept()\n")
	    continue
	}
	
	go handle_connection(conn)
    }
}

//
// Handle a connection from an agent
//

func handle_connection(c net.Conn) {
    var myDSN string
    var m Message

    input := bufio.NewScanner(c)
    
    for input.Scan() {
        inp := input.Text()

        ins := []byte(inp)
        err := json.Unmarshal(ins, &m)
        if (err != nil) {
	    log.Fatalf("Fatal attempting to unmarshal JSON")
        }

	//
	// The DSN used to connect to the database should look like this:
	//   hostmon:xyzzy123@tcp(192.168.1.253:3306)/hostmonitor
	//
	
        myDSN = g_dbUser + ":" + g_dbPass + "@tcp(" + g_dbHost + ":3306)/" + g_dbName

        dbconn, dbConnErr := sql.Open("mysql", myDSN)
	
	if dbConnErr != nil {
	    log.Fatalf("Fatal connecting to database\n")
	}
	
	//
	// Test the database connection to make sure that we're in business.
	//
	
	dbPingErr := dbconn.Ping()
	if dbPingErr != nil {
	    log.Fatalf("Fatal attempting to ping database\n")
	}

        //
	// Check to see if the host exists in the host tracking table
	//

        dbCmd := "SELECT COUNT(*) FROM hosts where host = '" + m.Hostname + "';"
	_, dbExecErr := dbconn.Exec(dbCmd)
	if dbExecErr != nil {
	    log.Fatalf("Failed executing SELECT for host " + m.Hostname)
        }

	var hostp string
	_ = dbconn.QueryRow(dbCmd).Scan(&hostp)
	hostpi, _ := strconv.Atoi(hostp)

	//
	// If not, add it to the hosts table. MySQL will generate an ID
	//

        if (hostpi == 0) {
	    dbCmd = "INSERT INTO hosts (host) VALUES ('" + m.Hostname + "');"
	    _, dbExecErr = dbconn.Exec(dbCmd)
	    if dbExecErr != nil {
	        log.Fatalf("Failed executing host table INSERT for host " + m.Hostname)
            }
	}

        //
	// Retrieve previous set of data points for this host from the reports
	//  table
	//
	
	dbCmd = "SELECT * from reports where hostname = '" + m.Hostname + "' ORDER BY timestamp DESC LIMIT 1;"

        //
	// Note regaarding db.QueryRow(): We should know how many fields we
	//  have in the table. For each field, specify a parameter to the
	//  QueryRow().Scan() method. i.e.
	//      db.QueryRow(cmd).Scan(&f1, &f2, &f3, &f4) and so on
        //

	var dbTimeStamp, dbHostName, dbKernelVer, dbUptime, dbNumCPUs, dbPhysMem, dbLoadOne, dbLoadFive, dbLoadFifteen, dbSwapPctUsed, dbDiskReport string
	
	queryErr := dbconn.QueryRow(dbCmd).Scan(&dbTimeStamp, &dbHostName, &dbKernelVer, &dbUptime, &dbNumCPUs, &dbPhysMem, &dbLoadOne, &dbLoadFive, &dbLoadFifteen, &dbSwapPctUsed, &dbDiskReport)

        switch {
	    // If this happens, first database entry for the host in question
	    case queryErr == sql.ErrNoRows:
	        log.Printf("No rows returned executing SELECT for host %s\n", m.Hostname)
	    case queryErr != nil:
	        dbconn.Close()
	        log.Fatalf("Fatal attempting to execute SELECT for host %s\n", m.Hostname)
	    default:
	}

        //
	// Insert the data points from the current report into the database.
        //

	dbCmd = "INSERT INTO reports VALUES (" + strconv.FormatInt(m.Timestamp, 10) + ",'" + m.Hostname + "','" + m.KernelVer + "','" + m.Uptime + "','" + strconv.FormatInt(m.NumCPUs, 10) + "','" + strconv.FormatInt(m.Memtotal, 10) + "','" + strconv.FormatFloat(m.LoadOne, 'f', 6, 64) + "','" + strconv.FormatFloat(m.LoadFive, 'f', 6, 64) + "','" + strconv.FormatFloat(m.LoadFifteen, 'f', 6, 64) + "','" + strconv.FormatFloat(m.SwapUsed, 'f', 6, 64) + "','" + m.DiskReport + "');"

	_, dbExecErr = dbconn.Exec(dbCmd)
	if dbExecErr != nil {
	    dbconn.Close()
	    log.Fatalf("Fatal executing reports table INSERT for host %s\n", m.Hostname)
	}
	
	dbconn.Close()
	
	//
	// Now we have historic (from the database) and current (from the current connection) data points and we
	// can act on these i.e. calculate differentials and send notifications.
	//
	
	dbLoadOneF,_ := strconv.ParseFloat(dbLoadOne, 64)
	dbSwapPctUsedF, _ := strconv.ParseFloat(dbSwapPctUsed, 64)
	
	loadDifferential := math.Abs(dbLoadOneF-m.LoadOne)
	swapDifferential := math.Abs(dbSwapPctUsedF-m.SwapUsed)
	
	//
	// Look at system load for this host and send notification if the threshold is exceeded. We only consider situations where the new load is
	//  greater than the old load to be actionable, no sense in messaging on a load DECREASE.
	//

        if (m.LoadOne > dbLoadOneF) {
	    if ((m.LoadOne > g_loadThreshold) && (loadDifferential > g_loadFirstDThreshold)) {
	        lo := strconv.FormatFloat(m.LoadOne, 'f', 6, 64)
	        send_email_notification("Subject: System load warning on " + m.Hostname, "System load has reached " + lo + " from " + dbLoadOne)
	    }
	}

        //
	// Look at swap utilization for this host and send notification if the threshold is exceeded. Again, we only consider the situation where the
	//  new swap utilization is greater than the old utilization to be actionable, no sense in messging on swap utilization DECREASE.
	//
	
	if (m.SwapUsed > dbSwapPctUsedF) {
	    if ((m.SwapUsed > g_swapThreshold) && (swapDifferential > g_swapFirstDThreshold)) {
	        su := strconv.FormatFloat(m.SwapUsed, 'f', 6, 64)
	        send_email_notification("Subject: Swap utilization warning on " + m.Hostname, "Swap utilization has reached " + su + "% from " + dbSwapPctUsed + "%")	
	    }
	}
	
        //
	// Now let's look at the disk utilization report for this host and send an alert if the threshold is exceeded. Since disk utilization usually
	//  varies at a much slower rate than system load or swap consumption, we notify for disk only at specified intervals, not every run.
	//
	
        diskReptComponents := strings.Fields(m.DiskReport)
	
	for i := 0; i < len(diskReptComponents)-1; i++ {
	    valueToTest, _ := strconv.ParseInt(diskReptComponents[i+1], 10, 64)
	    
	    if ((valueToTest >= g_diskThreshold) && (math.Abs(float64(time.Now().Unix() - lastDNotify[m.Hostname])) >= float64(g_diskReportInterval))) {
	        send_email_notification("Subject: Disk utilization warning on " + m.Hostname, "Disk utilization on " + diskReptComponents[i] + " has reached " + diskReptComponents[i+1] + "%")
		lastDNotify[m.Hostname] = time.Now().Unix()
	    }
	}	
    }
    
    c.Close()
}

//
// Send a notification e-mail
//

func send_email_notification(subj string, body string) {
    eMailConn, eMailErr := smtp.Dial("localhost:25")
    if eMailErr != nil {
        log.Printf("SMTP server connection failure sending notification\n")
    }
		
    eMailConn.Mail(g_eMailFrom)
    eMailConn.Rcpt(g_eMailTo)
		
    wc, eMailErr := eMailConn.Data()
    if eMailErr != nil {
        log.Printf("Failure initiating DATA stage sending notification\n")
    }
		
    defer wc.Close()
		
    buf := bytes.NewBufferString("From: " + g_eMailFrom + "\r\n" + "To: " + g_eMailTo + "\r\n" + subj + "\r\n\r\n" + body + "\r\n")
		
    _, eMailErr = buf.WriteTo(wc)
    if eMailErr != nil {
        log.Printf("Failure writing notification message DATA\n")
    }
}
