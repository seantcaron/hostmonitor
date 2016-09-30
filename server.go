//
// Host monitor data collection server, Sean Caron, scaron@umich.edu
//

package main

import (
    // "io"
    "net"
    // "os"
    "fmt"
    "strings"
    "strconv"
    "bufio"
    "math"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "net/smtp"
    "bytes"
)

func main() {

    listener, err := net.Listen("tcp", "localhost:5962")
    if err != nil {
        return
    }
    
    for {
        conn, err := listener.Accept()
	if err != nil {
	    continue
	}
	
	go handle_connection(conn)
    }
}

// temporary database for testing:
// create table reports (timestamp bigint, hostname varchar(255), numcpus varchar(255), physmem varchar(255), loadone varchar(255), loadfive varchar(255), loadfifteen varchar(255), swapused varchar(255), diskreport varchar(255));
//

func handle_connection(c net.Conn) {

    var dbUser string = "hostmon"
    var dbPass string = "xyzzy123"
    var dbName string = "hostmonitor"
    var dbHost string = "localhost"

    //var loadThreshold float64 = 35.0
    var loadThreshold float64 = 0.01
    var swapThreshold float64 = 30.0
    
    //var loadFirstDThreshold float64 = 10.0
    var loadFirstDThreshold float64 = 0.01
    var swapFirstDThreshold float64 = 5.0
    
    var myDSN string;
    
    eMailTo := []string{"scaron@umich.edu"}
    eMailFrom := []string{"do-not-reply@umich.edu"}
    
    input := bufio.NewScanner(c)
    
    fmt.Printf("%s\n", input.Text())
    
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
	
	// The DSN used to connect to the database should look like this:
	// hostmon:xyzzy123@tcp(192.168.1.253:3306)/hostmonitor
	
        myDSN = dbUser + ":" + dbPass + "@tcp(" + dbHost + ":3306)/" + dbName
    
        fmt.Printf("DEBUG: Attempting to connect with DSN: %s\n", myDSN)
	
        dbconn, dbConnErr := sql.Open("mysql", myDSN)
	
	if dbConnErr != nil {
	    fmt.Printf("ERROR connecting to database!\n")
	}
	
	// Test the connection and make sure we're in business
	dbPingErr := dbconn.Ping()
	if dbPingErr != nil {
	    fmt.Printf("ERROR attempting to ping database connection!\n")
	}
	
	// Prepare the command to retrieve the previous set of data points for this host
	dbCmd := "SELECT * from reports where hostname = '" + hostName + "' ORDER BY timestamp DESC LIMIT 1;"
	fmt.Printf("Attempting to execute:\n%s\n", dbCmd)



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
	        fmt.Printf("ERROR: No rows returned by the SELECT!\n")
	    case queryErr != nil:
	        fmt.Printf("ERROR: Some other error occurred executing the SELECT!\n")
	    default:
	        fmt.Printf("Retrieved: %s %s %s %s %s\n", dbTimeStamp, dbHostName, dbLoadOne, dbSwapPctUsed, dbDiskReport)
	}

	// Prepare the command to insert the newest set of data points
	
	dbCmd = "INSERT INTO reports VALUES (" + timeStamp + ",'" + hostName + "','" + numCPUs + "','" + physMem + "','" + loadOne + "','" + loadFive + "','" + loadFifteen + "','" + swapPctUsed + "','" + diskReport + "');"
	
	fmt.Printf("Attempting to execute:\n%s\n", dbCmd)
	
	_, dbExecErr := dbconn.Exec(dbCmd)
	if dbExecErr != nil {
	    fmt.Printf("ERROR executing insert statement!\n")
	}
	
	//
	// Now we have historic and current data points so we can act on these i.e. calculate a differential and send notifications
	//
	
	dbLoadOneF,_ := strconv.ParseFloat(dbLoadOne, 64)
	loadOneF, _ := strconv.ParseFloat(loadOne, 64)
	dbSwapPctUsedF, _ := strconv.ParseFloat(dbSwapPctUsed, 64)
	swapPctUsedF, _ := strconv.ParseFloat(swapPctUsed, 64)
	
	loadDifferential := math.Abs(dbLoadOneF-loadOneF)
	swapDifferential := math.Abs(dbSwapPctUsedF-swapPctUsedF)
	
	fmt.Printf("Load diff: %f Swap diff: %f\n", loadDifferential, swapDifferential)
	
	// Send load notification e-mail	
	if ((loadOneF > loadThreshold) && (loadDifferential > loadFirstDThreshold)) {
	    eMailConn, eMailErr := smtp.Dial("localhost:25")
	    if eMailErr != nil {
	        fmt.Printf("ERROR sending load notification e-mail!\n")
	    }

            
	    eMailConn.Mail(strings.Join(eMailFrom,""))
	    eMailConn.Rcpt(strings.Join(eMailTo,""))
	    wc, eMailErr := eMailConn.Data()
	    if eMailErr != nil {
	        fmt.Printf("ERROR sending load notification e-mail!\n")
	    }
	    
	    defer wc.Close()
	    
	    buf := bytes.NewBufferString("From: " + strings.Join(eMailFrom,"") + "\r\n" + "To: " + strings.Join(eMailTo,"") + "\r\n" + "Subject: System load warning on " + hostName + "\r\n\r\n" + "System load has reached " + loadOne + "\r\n")
	    
	    _, eMailErr = buf.WriteTo(wc)
	    if eMailErr != nil {
	        fmt.Printf("ERROR sending load notification e-mail!\n")
	    }
	    
	}
	
	// Send swap notification e-mail
	if ((swapPctUsedF > swapThreshold) && (swapDifferential > swapFirstDThreshold)) {
	    eMailConn, eMailErr := smtp.Dial("localhost:25")
	    if eMailErr != nil {
	        fmt.Printf("ERROR sending swap notification e-mail!\n")
	    }
	    
	    eMailConn.Mail(strings.Join(eMailFrom,""))
	    eMailConn.Rcpt(strings.Join(eMailTo,""))
	    
	    wc, eMailErr := eMailConn.Data()
	    if eMailErr != nil {
	        fmt.Printf("ERROR sending swap notification e-mail!\n")
	    }
	    
	    defer wc.Close()
	    
	    buf := bytes.NewBufferString("From: " + strings.Join(eMailFrom,"") + "\r\n" + "To: " + strings.Join(eMailTo,"") + "\r\n" + "Subject: Swap utilization warning on " + hostName + "\r\n\r\n" + "Swap utilization has reached " + swapPctUsed + "%\r\n")
	    
	    _, eMailErr = buf.WriteTo(wc)
	    if eMailErr != nil {
	        fmt.Printf("ERROR sending swap notification e-mail!\n")
            }			
	}
	
	//
	// Update swap differential per-host table, and disk utilization per-host table
	//
	
	dbconn.Close()
    }
    
    c.Close()
}
