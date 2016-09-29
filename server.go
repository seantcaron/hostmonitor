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
	
        //fmt.Printf("%s\n", input.Text())
    
        fmt.Printf("Got report from %s on %s\n", hostName, timeStamp)
	fmt.Printf("numCPUs: %s physMem: %s\n", numCPUs, physMem)
	fmt.Printf("Load averages: %s %s %s\n", loadOne, loadFive, loadFifteen)
	fmt.Printf("Swap percent used: %s\n", swapPctUsed)
	fmt.Printf("%s\n", diskReport)
	
	// The DSN used to connect to the database should look like this:
	// hostmon:xyzzy123@tcp(192.168.1.253:3306)/hostmonitor
	
        myDSN = dbUser + ":" + dbPass + "@tcp(" + dbHost + ":3306)/" + dbName
    
        fmt.Printf("DEBUG: Attempting to connect with DSN: %s\n", myDSN)
	
        dbconn, dbConnErr := sql.Open("mysql", myDSN)
	
	if dbConnErr != nil {
	    fmt.Printf("ERROR connecting to database!\n")
	}
	
	dbPingErr := dbconn.Ping()
	if dbPingErr != nil {
	    fmt.Printf("ERROR attempting to ping database connection!\n")
	}
	
	dbCmd := "INSERT INTO reports VALUES ('" + timeStamp + "','" + hostName + "','" + numCPUs + "','" + physMem + "','" + loadOne + "','" + loadFive + "','" + loadFifteen + "','" + swapPctUsed + "','" + diskReport + "');"
	
	fmt.Printf("Attempting to execute:\n%s\n", dbCmd)
	
	_, dbExecErr := dbconn.Exec(dbCmd)
	if dbExecErr != nil {
	    fmt.Printf("ERROR executing insert statement!\n")
	}
	
	dbconn.Close()
    }
    
    c.Close()
}
