//
// Host monitor data collection server
//  Sean Caron scaron@umich.edu
//

package main

import (
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
  "encoding/json"
  "net/http"
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

var dbconn *sql.DB

func main() {
  //var bindaddr, conffile string
  var conffile string

  if (len(os.Args) != 5) {
    log.Fatalf("Usage: %s -b bindaddr -f configfile", os.Args[0])
  }

  for i := 1; i < len(os.Args); i++ {
    switch os.Args[i] {
      //case "-b":
        //bindaddr = os.Args[i+1]
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
  // The DSN used to connect to the database should look like this:
  //   hostmon:xyzzy123@tcp(192.168.1.253:3306)/hostmonitor
  //

  myDSN := g_dbUser + ":" + g_dbPass + "@tcp(" + g_dbHost + ":3306)/" + g_dbName

  // When dbconn is global, this needs to be =, not := !!
  dbconn, err = sql.Open("mysql", myDSN)

  if err != nil {
    log.Fatalf("Fatal connecting to database\n")
  }

  //
  // Test the database connection to make sure that we're in business.
  //

  err = dbconn.Ping()
  if err != nil {
    log.Fatalf("Fatal attempting to ping database")
  }

  //
  // Start notifier Goroutine
  //

  go task_scan_and_notify()

  //
  // Start listening for connections from the dashboard
  //

  http.HandleFunc("/host/", task_handle_host)
  http.ListenAndServe(":8962", nil)

  dbconn.Close()
}

//
// Handle a connection
//

func task_handle_host(w http.ResponseWriter, r *http.Request) {
  var m Message

  // Extract hostname component of the path and the method
  h := r.URL.Path[len("/host/"):]
  me := r.Method

  log.Printf("Got host %s (len=%d) with method %s\n", h, len(h), me)

  // We will key off r.Method = "GET" or "POST"

  // /host/        GET -> list all POST -> do nothing
  // /host/name    GET -> list one POST -> update (or create) one

  switch me {
    case "GET":
      if (len(h) == 0) {
        // If we get no host parameter, we'll dump the whole list, so, first
        //  execute (1) and for each result in (1) execute (2).
        rs, er := dbconn.Query("SELECT host from hosts ORDER BY host ASC")
        if (er != nil) {
          http.Error(w, "Fatal attempting to dump hosts", http.StatusInternalServerError)
        }

        var hh string
        for rs.Next() {
          er = rs.Scan(&hh)
          if (er != nil) {
            http.Error(w, "Fatal attempting to dump hosts", http.StatusInternalServerError)
          }
          dbCmd_2 := "SELECT * FROM reports WHERE hostname = '" + hh + "' ORDER BY timestamp DESC LIMIT 1;"
          qe := dbconn.QueryRow(dbCmd_2).Scan(&m.Timestamp, &m.Hostname, &m.KernelVer, &m.Release, &m.Uptime,
            &m.NumCPUs, &m.Memtotal, &m.LoadOne, &m.LoadFive, &m.LoadFifteen, &m.SwapUsed, &m.DiskReport)
          if (qe  != nil) {
            http.Error(w, "Fatal attempting to dump hosts", http.StatusInternalServerError)
          }
          rp, erro := json.Marshal(m)
          if (erro != nil) {
            http.Error(w, "Fatal attempting to marshal JSON", http.StatusInternalServerError)
          }
          fmt.Fprintf(w, "%s", rp)
        }
      } else {
        // When we do have a host, just grab the most recent line for that host.
        dbCmd := "SELECT * from reports where hostname = '" + h + "' ORDER BY timestamp DESC LIMIT 1;"

        //
        // For each field, specify a parameter to QueryRow().Scan() i.e.
        //  db.QueryRow(cmd).Scan(&f1, &f2, &f3, &f3) and so on
        //

        queryErr := dbconn.QueryRow(dbCmd).Scan(&m.Timestamp, &m.Hostname, &m.KernelVer, &m.Release, &m.Uptime,
          &m.NumCPUs, &m.Memtotal, &m.LoadOne, &m.LoadFive, &m.LoadFifteen, &m.SwapUsed, &m.DiskReport)

        switch {
          case queryErr == sql.ErrNoRows:
            http.Error(w, "No such host " + h, http.StatusNotFound)
            return
          case queryErr != nil:
            dbconn.Close()
            http.Error(w, "Fatal attempting to execute SELECT for host " + h, http.StatusInternalServerError)
            return
          default:
        }
        rpt, err := json.Marshal(m)

        if (err != nil) {
          http.Error(w, "Fatal attempting to marshal JSON", http.StatusInternalServerError)
          return
        }

        fmt.Fprintf(w, "%s", rpt)
      }
  case "POST":
    if (len(h) == 0) {
      http.Error(w, "Must specify a host for a POST request", http.StatusInternalServerError)
    }

    // Must call ParseForm() before accessing elements
    r.ParseForm()
    //bb := r.Form

    // Populate message Fields
    m.Timestamp, _ = strconv.ParseInt(r.FormValue("Timestamp"), 10, 64)
    m.Hostname = r.FormValue("Hostname")
    m.NumCPUs, _ = strconv.ParseInt(r.FormValue("NumCPUs"), 10, 64)
    m.Memtotal, _ = strconv.ParseInt(r.FormValue("Memtotal"), 10, 64)
    m.LoadOne, _ = strconv.ParseFloat(r.FormValue("LoadOne"), 64)
    m.LoadFive, _ = strconv.ParseFloat(r.FormValue("LoadFive"), 64)
    m.LoadFifteen, _ = strconv.ParseFloat(r.FormValue("LoadFifteen"), 64)
    m.SwapUsed, _ = strconv.ParseFloat(r.FormValue("SwapUsed"), 64)
    m.KernelVer = r.FormValue("KernelVer")
    m.Release = r.FormValue("Release")
    m.Uptime = r.FormValue("Uptime")
    m.DiskReport = r.FormValue("DiskReport")

    //
    // Check to see if the host exists in the host tracking table
    //

    dbCmd := "SELECT COUNT(*) FROM hosts where host = '" + m.Hostname + "';"
  	_, dbExecErr := dbconn.Exec(dbCmd)
  	if dbExecErr != nil {
      http.Error(w, "Fatal executing select for host " + m.Hostname, http.StatusInternalServerError)
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
        http.Error(w, "Failed executing host table INSERT for host " + m.Hostname, http.StatusInternalServerError)
      }
    }

    //
    // Retrieve previous set of data points for this host from the reports
    //  table
    //

  	dbCmd = "SELECT * from reports where hostname = '" + m.Hostname + "' ORDER BY timestamp DESC LIMIT 1;"

    //
    // Note regarding db.QueryRow(): We should know how many fields we
  	//  have in the table. For each field, specify a parameter to the
  	//  QueryRow().Scan() method. i.e.
  	//      db.QueryRow(cmd).Scan(&f1, &f2, &f3, &f4) and so on
    //

    var dbTimeStamp, dbHostName, dbKernelVer, dbRelease, dbUptime, dbNumCPUs, dbPhysMem, dbLoadOne, dbLoadFive, dbLoadFifteen, dbSwapPctUsed, dbDiskReport string

    queryErr := dbconn.QueryRow(dbCmd).Scan(&dbTimeStamp, &dbHostName, &dbKernelVer, &dbRelease, &dbUptime,
      &dbNumCPUs, &dbPhysMem, &dbLoadOne, &dbLoadFive, &dbLoadFifteen, &dbSwapPctUsed, &dbDiskReport)

    switch {
  	    // If this happens, first database entry for the host in question
  	    case queryErr == sql.ErrNoRows:
  	        log.Printf("No rows returned executing SELECT for host %s\n", m.Hostname)
  	    case queryErr != nil:
  	        dbconn.Close()
            http.Error(w, "Fatal attempting to execute SELECT for host" + m.Hostname, http.StatusInternalServerError)
  	    default:
  	}

    //
  	// Insert the data points from the current report into the database.
    //

  	dbCmd = "INSERT INTO reports VALUES (" + strconv.FormatInt(m.Timestamp, 10) + ",'" + m.Hostname + "','" + m.KernelVer + "','" + m.Release + "','" + m.Uptime + "','" + strconv.FormatInt(m.NumCPUs, 10) + "','" + strconv.FormatInt(m.Memtotal, 10) + "','" + strconv.FormatFloat(m.LoadOne, 'f', 6, 64) + "','" + strconv.FormatFloat(m.LoadFive, 'f', 6, 64) + "','" + strconv.FormatFloat(m.LoadFifteen, 'f', 6, 64) + "','" + strconv.FormatFloat(m.SwapUsed, 'f', 6, 64) + "','" + m.DiskReport + "');"

    log.Printf("Attempting to execute: %s\n", dbCmd)
  	_, dbExecErr = dbconn.Exec(dbCmd)
  	if dbExecErr != nil {
  	    dbconn.Close()
        http.Error(w, "Fatal executing reports table INSERT for host " + m.Hostname, http.StatusInternalServerError)
  	}

    // r.Form is automatically a parsed map with appropriate keys and values
    //log.Printf("Got POST <%s>\n", bb)
    log.Printf("POST from: %s %s %s\n", m.Hostname, m.KernelVer, m.Release)

  }
}

//
// Scan hosts database at configured intervals and send notifications if
// thresholds have been exceeded.
//

func task_scan_and_notify() {
  t := time.NewTicker(time.Second*60) // Fixed for testing, configurable when done

  var htt []string

  for range t.C {
    // Dump the list of hosts
    rs, er := dbconn.Query("SELECT host from hosts ORDER BY host ASC")
    if (er != nil) {
      log.Fatalf("Fatal compiling list for scan and notify")
    }

    var hh string
    for rs.Next() {
      er = rs.Scan(&hh)
      if (er != nil) {
        log.Fatalf("Fatal compiling list for scan and notify")
      }

      htt = append(htt, hh)
    }

    // For each host, run checks and send notifications

    for c, _ := range htt {
      rss, err := dbconn.Query("SELECT * FROM reports WHERE hostname = '" + htt[c] + "' ORDER BY timestamp DESC LIMIT 2")
      if (err != nil) {
        log.Fatalf("Fatal attempting to scan and notify 1")
      }

      var f0, f1, f2, f3, f4, f5, f6, f7, f8, f9, f10, f11 string
      var f0h, f1h, f2h, f3h, f4h, f5h, f6h, f7h, f8h, f9h, f10h, f11h string
      // Collect data point 1 for this host (most recent)
      rss.Next()
      err = rss.Scan(&f0, &f1, &f2, &f3, &f4, &f5, &f6, &f7, &f8, &f9, &f10, &f11)
      if (err != nil) {
        log.Printf("Skipping inconsistent host " + htt[c] + ", host in hosts table but no reports found")
        continue
      }

      log.Printf("#1: %s %s %s %s %s", f0, f1, f2, f3, f4)

      // Collect data point 2 for this host (historical)
      rss.Next()
      err = rss.Scan(&f0h, &f1h, &f2h, &f3h, &f4h, &f5h, &f6h, &f7h, &f8h, &f9h, &f10h, &f11h)
      if (err != nil) {
        log.Printf("Only one record for host " + htt[c])
        continue
      }

      log.Printf("#2: %s %s %s %s %s", f0, f1, f2, f3, f4)

      lo, _ := strconv.ParseFloat(f7, 64)
      loh, _ := strconv.ParseFloat(f7h, 64)
      sw, _ := strconv.ParseFloat(f10, 64)
      swh, _ := strconv.ParseFloat(f10h, 64)

      dl := math.Abs(lo-loh)
      ds := math.Abs(sw-swh)

      log.Printf("%f %f", dl, ds)

      // Look at system load and notify on positive differential exceeding Thresholds
      if (lo > loh) {
        if ((lo > g_loadThreshold) && (dl > g_loadFirstDThreshold)) {
          send_email_notification("Subject: System load warning on " + htt[c], "System load has reached " + f7 + " from " + f7h)
        }
      }

      // Look at swap utilization and notify on positive differential exceeding thresholds
      if (sw > swh) {
        if ((sw > g_swapThreshold) && (ds > g_swapFirstDThreshold)) {
          send_email_notification("Subject: Swap utilization warning on " + htt[c], "Swap utilization has reached " + f10 + "% from " + f10h + "%")
        }
      }

      // Look at disk report and notify on threshold exceeded
      diskReptComponents := strings.Fields(f11)

      for i := 0; i < len(diskReptComponents)-1; i++ {
        valueToTest, _ := strconv.ParseInt(diskReptComponents[i+1], 10, 64)

        if ((valueToTest >= g_diskThreshold) && (math.Abs(float64(time.Now().Unix() - lastDNotify[htt[c]])) >= float64(g_diskReportInterval))) {
          send_email_notification("Subject: Disk utilization warning on " + htt[c], "Disk utilization on " + diskReptComponents[i] + " has reached " + diskReptComponents[i+1] + "%")
          lastDNotify[htt[c]] = time.Now().Unix()
        }
      }

    }

    //log.Printf("Host dump follows")
    //for c, _ := range htt {
    //  log.Printf("  %s", htt[c])
    //}

    htt = nil

  }
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
