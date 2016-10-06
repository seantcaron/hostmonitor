//
// Host monitor data collection server
//  Sean Caron scaron@umich.edu
//

package main

import (
    //"io"
    "net"
    "os"
    "fmt"
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
)

//
// Configuration parameters go in global variables.
//

var g_dbUser, g_dbPass, g_dbHost, g_dbName, g_eMailTo, g_eMailFrom string
var g_loadThreshold, g_swapThreshold, g_loadFirstDThreshold, g_swapFirstDThreshold float64
var g_diskThreshold int64

func main() {

    //
    // Open log file
    //
    
    logFile, err := os.Create("/var/log/hostmonitor.log")
    
    if err != nil {
        log.Fatalf("Failed opening log file for writing\n")
    }
    
    log_with_timestamp(logFile, "Host monitor data server starting up")
    
    //
    // Read in the configuration file.
    //
    
    haveParam := make(map[string]bool)
    
    confFile, err := os.Open("/etc/hostmonitor/server.conf")
    
    if err != nil {
        log_with_timestamp(logFile, "Failed opening configuration file for reading")
        logFile.Close()
        log.Fatalf("Failed opening configuration file for reading\n")
    }
    
    inp := bufio.NewScanner(confFile)
    
    for inp.Scan() {
        line := inp.Text()
	
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
	    default:
	        log_with_timestamp(logFile, "Ignoring nonsense configuration parameter " + theFields[1])
	}
    }
    
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
	(haveParam["diskThreshold"] != true)) {
	log_with_timestamp(logFile, "Fatal missing configuration directive")
	logFile.Close()
	log.Fatalf("Fatal missing configuration directive\n")
    }
    
    //timeStamp = time.Now().Unix()
    //fmt.Fprintf(theLog, "%d: Configuration report follows\n", timeStamp)
    
    //fmt.Fprintf(theLog, "            dbUser %s dbPass %s dbHost %s dbName %s\n", g_dbUser, g_dbPass, g_dbHost, g_dbName)
    //fmt.Fprintf(theLog, "            eMailTo %s eMailFrom %s\n", g_eMailTo, g_eMailFrom)
    //fmt.Fprintf(theLog, "            Thresholds: %f %f %f %f %d\n\n", g_loadThreshold, g_swapThreshold, g_loadFirstDThreshold, g_swapFirstDThreshold, g_diskThreshold)  
    //theLog.Flush()
    
    confFile.Close()
    
    //
    // Start listening for connections
    //
    
    listener, err := net.Listen("tcp", "localhost:5962")
    if err != nil {
        log_with_timestamp(logFile, "Failure calling net.Listen()")
	logFile.Close()
	log.Fatalf("Failure calling net.Listen()\n")
    }
    
    //
    // Spin off a new Goroutine for each connection
    //
    
    for {
        conn, err := listener.Accept()
	if err != nil {
	    log_with_timestamp(logFile, "Non zero value returned by listener.Accept()")
	    continue
	}
	
	go handle_connection(conn, logFile)
    }
}

//
// Schema:
//  timestamp bigint
//  hostname varchar(68)
//  numcpus varchar(8)
//  physmem varchar(16)
//  loadone varchar(12)
//  loadfive varchar(12)
//  loadfifteen varchar(12)
//  swapused varchar(12)
//  diskreport varchar(68)
//
//  CREATE TABLE reports (timestamp bigint, hostname varchar(68), numcpus varchar(8), physmem varchar(16), loadone varchar(12),
//    loadfive varchar(12), loadfifteen varchar(12), swapused varchar(12), diskreport varchar(68));
//



func handle_connection(c net.Conn, f *os.File) {

    var myDSN string;
    
    input := bufio.NewScanner(c)
    
    for input.Scan() {
    
        inp := input.Text()
	
	data := strings.Split(inp, ",")
	
	timeStamp := data[0]
	hostName := data[1]
	numCPUs := data[2]
	physMem := data[3]
	loadOne := data[4]
	loadFive := data[5]
	loadFifteen := data[6]
	swapPctUsed := data[7]
	diskReport := data[8]
	
	//
	// The DSN used to connect to the database should look like this:
	//   hostmon:xyzzy123@tcp(192.168.1.253:3306)/hostmonitor
	//
	
        myDSN = g_dbUser + ":" + g_dbPass + "@tcp(" + g_dbHost + ":3306)/" + g_dbName
    
        //logTimeStamp := time.Now().Unix()
        //fmt.Fprintf(log, "%d: Attempting to connect with DSN: %s\n", logTimeStamp, myDSN)
	//log.Flush()
	
        dbconn, dbConnErr := sql.Open("mysql", myDSN)
	
	if dbConnErr != nil {
	    log_with_timestamp(f, "Error connecting to database")
	    f.Close()
	    os.Exit(1)
	}
	
	//
	// Test the database connection to make sure that we're in business.
	//
	
	dbPingErr := dbconn.Ping()
	if dbPingErr != nil {
	    log_with_timestamp(f, "Error attempting to ping database connection")
	    f.Close()
	    os.Exit(1)
	}
	
	//
	// Retrieve the previous set of data points acquired for this host from the database.
	//
	
	dbCmd := "SELECT * from reports where hostname = '" + hostName + "' ORDER BY timestamp DESC LIMIT 1;"

        // I guess we can't use SELECT * with QueryRow, we need to SELECT a particular field from the row otherwise
	//  we will get an error, attempting to execute the QueryRow statement.
	// (We can, but we have to specify the correct number of fields in the Scan() call. If we only select one
	//  parameter, it works fine if we only specify one parameter to the Scan() function)
	//
	// We know how many fields we have up front, and we just specify N parameters to QueryRow().Scan() i.e.
	//  db.QueryRow(cmd).Scan(&f1, &f2, &f3, &f4) and so on
	
	var dbTimeStamp, dbHostName, dbNumCPUs, dbPhysMem, dbLoadOne, dbLoadFive, dbLoadFifteen, dbSwapPctUsed, dbDiskReport string
	
	queryErr := dbconn.QueryRow(dbCmd).Scan(&dbTimeStamp, &dbHostName, &dbNumCPUs, &dbPhysMem, &dbLoadOne, &dbLoadFive, &dbLoadFifteen, &dbSwapPctUsed, &dbDiskReport)
	
	switch {
	    // If this happens, first database entry for the host in question
	    case queryErr == sql.ErrNoRows:
	        log_with_timestamp(f, "No rows returned executing SELECT for host " + hostName)
	    case queryErr != nil:
	        log_with_timestamp(f, "Some other error occurred executing SELECT for host " + hostName)
		f.Close()
		dbconn.Close()
		os.Exit(1)
	    default:
	        continue
	}

        //
	// Insert the newest set of data points acquired for this host into the database.
	//
	
	dbCmd = "INSERT INTO reports VALUES (" + timeStamp + ",'" + hostName + "','" + numCPUs + "','" + physMem + "','" + loadOne + "','" + loadFive + "','" + loadFifteen + "','" + swapPctUsed + "','" + diskReport + "');"
	
	_, dbExecErr := dbconn.Exec(dbCmd)
	if dbExecErr != nil {
	    log_with_timestamp(f, "Failure executing INSERT for host " + hostName)
	    f.Close()
	    dbconn.Close()
	    os.Exit(1)
	}
	
	dbconn.Close()
	
	//
	// Now we have historic (from the database) and current (from the current connection) data points and we
	// can act on these i.e. calculate differentials and send notifications.
	//
	
	dbLoadOneF,_ := strconv.ParseFloat(dbLoadOne, 64)
	loadOneF, _ := strconv.ParseFloat(loadOne, 64)
	dbSwapPctUsedF, _ := strconv.ParseFloat(dbSwapPctUsed, 64)
	swapPctUsedF, _ := strconv.ParseFloat(swapPctUsed, 64)
	
	loadDifferential := math.Abs(dbLoadOneF-loadOneF)
	swapDifferential := math.Abs(dbSwapPctUsedF-swapPctUsedF)
	
	//
	// Look at system load for this host and send notification if the threshold is exceeded.
	//
		
	if ((loadOneF > g_loadThreshold) && (loadDifferential > g_loadFirstDThreshold)) {
	    eMailConn, eMailErr := smtp.Dial("localhost:25")
	    if eMailErr != nil {
	        log_with_timestamp(f, "SMTP server connection failure sending load notification")
	    }
       
	    eMailConn.Mail(g_eMailFrom)
	    eMailConn.Rcpt(g_eMailTo)
	    wc, eMailErr := eMailConn.Data()
	    if eMailErr != nil {
	        log_with_timestamp(f, "Failure initiating DATA stage sending load notification")
	    }
	    
	    defer wc.Close()
	    
	    buf := bytes.NewBufferString("From: " + g_eMailFrom + "\r\n" + "To: " + g_eMailTo + "\r\n" + "Subject: System load warning on " + hostName + "\r\n\r\n" + "System load has reached " + loadOne + "\r\n")
	    
	    _, eMailErr = buf.WriteTo(wc)
	    if eMailErr != nil {
	        log_with_timestamp(f, "Failure writing load notification message DATA")
	    }
	}
	
        //
	// Look at swap utilization for this host and send notification if the threshold is exceeded.
	//
	
	if ((swapPctUsedF > g_swapThreshold) && (swapDifferential > g_swapFirstDThreshold)) {
	    eMailConn, eMailErr := smtp.Dial("localhost:25")
	    if eMailErr != nil {
	        log_with_timestamp(f, "SMTP server connection failure sending swap notification")
	    }
	    
	    eMailConn.Mail(g_eMailFrom)
	    eMailConn.Rcpt(g_eMailTo)
	    
	    wc, eMailErr := eMailConn.Data()
	    if eMailErr != nil {
	        log_with_timestamp(f, "Failure initiating DATA stage sending swap notification")
	    }
	    
	    defer wc.Close()
	    
	    buf := bytes.NewBufferString("From: " + g_eMailFrom + "\r\n" + "To: " + g_eMailTo + "\r\n" + "Subject: Swap utilization warning on " + hostName + "\r\n\r\n" + "Swap utilization has reached " + swapPctUsed + "%\r\n")
	    
	    _, eMailErr = buf.WriteTo(wc)
	    if eMailErr != nil {
	        log_with_timestamp(f, "Failure writing swap notification message DATA")
            }			
	}
	
        //
	// Now let's look at the disk utilization report for this host and send an alert if the threshold
	// is exceeded.
	//
	
        diskReptComponents := strings.Fields(diskReport)
	
	for i := 0; i < len(diskReptComponents)-1; i++ {
	
	    valueToTest, _ := strconv.ParseInt(diskReptComponents[i+1], 10, 64)
	    
	    if valueToTest >= g_diskThreshold {
	        eMailConn, eMailErr := smtp.Dial("localhost:25")
		if eMailErr != nil {
		    log_with_timestamp(f, "SMTP server connection failure sending disk notification")
		}
		
		eMailConn.Mail(g_eMailFrom)
		eMailConn.Rcpt(g_eMailTo)
		
		wc, eMailErr := eMailConn.Data()
		if eMailErr != nil {
		    log_with_timestamp(f, "Failure initiating DATA stage sending disk notification")
		}
		
		defer wc.Close()
		
		buf := bytes.NewBufferString("From: " + g_eMailFrom + "\r\n" + "To: " + g_eMailTo + "\r\n" + "Subject: Disk utilization warning on " + hostName + "\r\n\r\n" + "Disk utilization on " + diskReptComponents[i] + " has reached " + diskReptComponents[i+1] + "%\r\n")
		
		_, eMailErr = buf.WriteTo(wc)
		if eMailErr != nil {
		    log_with_timestamp(f, "Failure writing disk notification message DATA")
		}
	    }
	}
	
	
    }
    
    f.Close()
    
    c.Close()
}

//
// Write a message to the server log with a timestamp
//

func log_with_timestamp(f *os.File, s string) {
    l := bufio.NewWriter(f)
    
    stamp := time.Now().Unix()
    fmt.Fprintf(l, "%d: %s\n", stamp, s)
    l.Flush()   
}
