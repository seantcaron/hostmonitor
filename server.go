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
    "bufio"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
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
// create table reports (timestamp varchar(255), hostname varchar(255), numcpus varchar(255), physmem varchar(255), loadone varchar(255), loadfive varchar(255), loadfifteen varchar(255), swapused varchar(255), diskreport varchar(255));
//

func handle_connection(c net.Conn) {

    var dbUser string = "hostmon"
    var dbPass string = "xyzzy123"
    var dbName string = "hostmonitor"
    var dbHost string = "192.168.1.253"

    var myDSN string;
    
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
	dbCmd := "SELECT timestamp from reports where hostname = '" + hostName + "' ORDER BY timestamp DESC LIMIT 1;"
	fmt.Printf("Attempting to execute:\n%s\n", dbCmd)



        // I guess we can't use SELECT * with QueryRow, we need to SELECT a particular field from the row otherwise
	//  we will get an error, attempting to execute the QueryRow statement.
	
        var result string
	queryErr := dbconn.QueryRow(dbCmd).Scan(&result)
	switch {
	    case queryErr == sql.ErrNoRows:
	        fmt.Printf("ERROR: No rows returned by the SELECT!\n")
	    case queryErr != nil:
	        fmt.Printf("ERROR: Some other error occurred executing the SELECT!\n")
	    default:
	        fmt.Printf("Retrieved: %s\n", result)
	}



	

	// Prepare the command to insert the newest set of data points
	
	dbCmd = "INSERT INTO reports VALUES (" + timeStamp + ",'" + hostName + "','" + numCPUs + "','" + physMem + "','" + loadOne + "','" + loadFive + "','" + loadFifteen + "','" + swapPctUsed + "','" + diskReport + "');"
	
	fmt.Printf("Attempting to execute:\n%s\n", dbCmd)
	
	_, dbExecErr := dbconn.Exec(dbCmd)
	if dbExecErr != nil {
	    fmt.Printf("ERROR executing insert statement!\n")
	}
	
	dbconn.Close()
    }
    
    c.Close()
}
