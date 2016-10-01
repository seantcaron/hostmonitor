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
    
    theLog := bufio.NewWriter(logFile)
    
    timeStamp := time.Now().Unix()
    
    fmt.Fprintf(theLog, "%d: Host monitor data server starting up\n", timeStamp)
    theLog.Flush()
    
    //
    // Read in the configuration file.
    //
    
    haveParam := make(map[string]bool)
    
    confFile, err := os.Open("/etc/hostmonitor/server.conf")
    
    if err != nil {
        timeStamp := time.Now().Unix()
        fmt.Fprintf(theLog, "%d: Failed opening configuration file for reading\n", timeStamp)
	theLog.Flush()
        logFile.Close()
        log.Fatalf("Failed opening configuration file for reading\n")
    }
    
    inp := bufio.NewScanner(confFile)
    
    for inp.Scan() {
        line := inp.Text()
	
	theFields := strings.Fields(line)
	
	haveParam[theFields[0]] = true
	
	if (theFields[0] == "dbUser") {
	    g_dbUser = theFields[1]
	}
	
	if (theFields[0] == "dbPass") {
	    g_dbPass = theFields[1]
	}
	
	if (theFields[0] == "dbHost") {
	    g_dbHost = theFields[1]
	}
	
	if (theFields[0] == "dbName") {
	    g_dbName = theFields[1]
	}
	
	if (theFields[0] == "eMailTo") {
	    g_eMailTo = theFields[1]
	}
	
	if (theFields[0] == "eMailFrom") {
	    g_eMailFrom = theFields[1]
	}
	
	if (theFields[0] == "loadThreshold") {
	    g_loadThreshold, _ = strconv.ParseFloat(theFields[1], 64)
	}
	
	if (theFields[0] == "swapThreshold") {
	    g_swapThreshold, _ = strconv.ParseFloat(theFields[1], 64)
	}
	
	if (theFields[0] == "loadFirstDThreshold") {
	    g_loadFirstDThreshold, _ = strconv.ParseFloat(theFields[1], 64)
	}
	
	if (theFields[0] == "swapFirstDThreshold") {
	    g_swapFirstDThreshold, _ = strconv.ParseFloat(theFields[1], 64)
	}
	
	if (theFields[0] == "diskThreshold") {
	    g_diskThreshold, _ = strconv.ParseInt(theFields[1], 10, 64)
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
        timeStamp = time.Now().Unix()
	fmt.Fprintf(theLog, "%d: Fatal missing configuration directive\n")
	theLog.Flush()
	logFile.Close()
	log.Fatalf("Fatal missing configuration directive\n")
    }
    
    timeStamp = time.Now().Unix()
    fmt.Fprintf(theLog, "%d: Configuration report follows\n", timeStamp)
    
    fmt.Fprintf(theLog, "            dbUser %s dbPass %s dbHost %s dbName %s\n", g_dbUser, g_dbPass, g_dbHost, g_dbName)
    fmt.Fprintf(theLog, "            eMailTo %s eMailFrom %s\n", g_eMailTo, g_eMailFrom)
    fmt.Fprintf(theLog, "            Thresholds: %f %f %f %f %d\n\n", g_loadThreshold, g_swapThreshold, g_loadFirstDThreshold, g_swapFirstDThreshold, g_diskThreshold)  
    theLog.Flush()
    
    confFile.Close()
    
    //
    // Start listening for connections
    //
    
    listener, err := net.Listen("tcp", "localhost:5962")
    if err != nil {
        timeStamp = time.Now().Unix()
	fmt.Fprintf(theLog, "%d: Failure calling net.Listen()\n")
	theLog.Flush()
	logFile.Close()
	log.Fatalf("Failure calling net.Listen()\n")
    }
    
    //
    // Spin off a new Goroutine for each connection
    //
    
    for {
        conn, err := listener.Accept()
	if err != nil {
	    timeStamp = time.Now().Unix()
	    fmt.Fprintf(theLog, "%d: Non zero value returned by listener.Accept()\n")
	    theLog.Flush()
	    continue
	}
	
	go handle_connection(conn, logFile)
    }
}

//
// temporary database for testing:
// create table reports (timestamp bigint, hostname varchar(255), numcpus varchar(255), physmem varchar(255), loadone varchar(255), loadfive varchar(255), loadfifteen varchar(255), swapused varchar(255), diskreport varchar(255));
//

func handle_connection(c net.Conn, f *os.File) {

    var myDSN string;
    
    log := bufio.NewWriter(f)
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
    
        logTimeStamp := time.Now().Unix()
        fmt.Fprintf(log, "%d: Attempting to connect with DSN: %s\n", logTimeStamp, myDSN)
	log.Flush()
	
        dbconn, dbConnErr := sql.Open("mysql", myDSN)
	
	if dbConnErr != nil {
	    logTimeStamp = time.Now().Unix()
	    fmt.Fprintf(log, "%d: Error connecting to database\n")
	    log.Flush()
	    f.Close()
	    os.Exit(1)
	}
	
	//
	// Test the database connection to make sure that we're in business.
	//
	
	dbPingErr := dbconn.Ping()
	if dbPingErr != nil {
	    logTimeStamp = time.Now().Unix()
	    fmt.Fprintf(log, "%d: Error attempting to ping database connection\n")
	    log.Flush()
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
	
	logTimeStamp = time.Now().Unix()
	switch {
	    // If this happens, first database entry for the host in question
	    case queryErr == sql.ErrNoRows:
	        fmt.Fprintf(log, "%d: No rows returned executing the SELECT\n", logTimeStamp)
		log.Flush()
	    case queryErr != nil:
		fmt.Fprintf(log, "%d: Some other error occurred executing the SELECT\n", logTimeStamp)
		log.Flush()
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
	    logTimeStamp = time.Now().Unix()
	    fmt.Fprintf(log, "%d: Failure executing INSERT statement\n", logTimeStamp)
	    log.Flush()
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
	        logTimeStamp = time.Now().Unix()
		fmt.Fprintf(log, "%d: SMTP server connection failure sending load notification\n", logTimeStamp)
		log.Flush()
	    }
       
	    eMailConn.Mail(g_eMailFrom)
	    eMailConn.Rcpt(g_eMailTo)
	    wc, eMailErr := eMailConn.Data()
	    if eMailErr != nil {
	        logTimeStamp = time.Now().Unix()
		fmt.Fprintf(log, "%d: Failure initiating DATA stage of sending load notification\n", logTimeStamp)
		log.Flush()
	    }
	    
	    defer wc.Close()
	    
	    buf := bytes.NewBufferString("From: " + g_eMailFrom + "\r\n" + "To: " + g_eMailTo + "\r\n" + "Subject: System load warning on " + hostName + "\r\n\r\n" + "System load has reached " + loadOne + "\r\n")
	    
	    _, eMailErr = buf.WriteTo(wc)
	    if eMailErr != nil {
	        logTimeStamp = time.Now().Unix()
		fmt.Fprintf(log, "%d: Failure writing load notification message DATA\n", logTimeStamp)
		log.Flush()
	    }
	}
	
        //
	// Look at swap utilization for this host and send notification if the threshold is exceeded.
	//
	
	if ((swapPctUsedF > g_swapThreshold) && (swapDifferential > g_swapFirstDThreshold)) {
	    eMailConn, eMailErr := smtp.Dial("localhost:25")
	    if eMailErr != nil {
	        logTimeStamp = time.Now().Unix()
		fmt.Fprintf(log, "%d: SMTP server connection failure sending swap notification\n", logTimeStamp)
		log.Flush()
	    }
	    
	    eMailConn.Mail(g_eMailFrom)
	    eMailConn.Rcpt(g_eMailTo)
	    
	    wc, eMailErr := eMailConn.Data()
	    if eMailErr != nil {
	        logTimeStamp = time.Now().Unix()
		fmt.Fprintf(log, "%d: Failure initiating DATA stage of sending swap notification\n", logTimeStamp)
		log.Flush()
	    }
	    
	    defer wc.Close()
	    
	    buf := bytes.NewBufferString("From: " + g_eMailFrom + "\r\n" + "To: " + g_eMailTo + "\r\n" + "Subject: Swap utilization warning on " + hostName + "\r\n\r\n" + "Swap utilization has reached " + swapPctUsed + "%\r\n")
	    
	    _, eMailErr = buf.WriteTo(wc)
	    if eMailErr != nil {
	        logTimeStamp = time.Now().Unix()
		fmt.Fprintf(log, "%d: Failure writing swap notification message DATA\n", logTimeStamp)
		log.Flush()
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
		    logTimeStamp = time.Now().Unix()
		    fmt.Fprintf(log, "%d: SMTP server connection failure sending disk notification\n", logTimeStamp)
		    log.Flush()
		}
		
		eMailConn.Mail(g_eMailFrom)
		eMailConn.Rcpt(g_eMailTo)
		
		wc, eMailErr := eMailConn.Data()
		if eMailErr != nil {
		    logTimeStamp = time.Now().Unix()
		    fmt.Fprintf(log, "%d: Failure initiating DATA stage of sending disk notification\n", logTimeStamp)
		    log.Flush()
		}
		
		defer wc.Close()
		
		buf := bytes.NewBufferString("From: " + g_eMailFrom + "\r\n" + "To: " + g_eMailTo + "\r\n" + "Subject: Disk utilization warning on " + hostName + "\r\n\r\n" + "Disk utilization on " + diskReptComponents[i] + " has reached " + diskReptComponents[i+1] + "%\r\n")
		
		_, eMailErr = buf.WriteTo(wc)
		if eMailErr != nil {
		    logTimeStamp = time.Now().Unix()
		    fmt.Fprintf(log, "%d: Failure writing disk notification message DATA\n", logTimeStamp)
		    log.Flush()
		}
	    }
	}
	
	
    }
    
    f.Close()
    
    c.Close()
}
