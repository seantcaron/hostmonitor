Host Monitor
------------

Sean Caron
scaron@umich.edu

This is a simple client-server health monitor written in Go.

A small agent runs out of cron the client at any user desired interval and
collects the following data points:

* Timestamp
* Host name
* Number of installed CPUs
* Total physical memory
* Load averages
* Percentage of swap used
* Disk utilization report

A server runs on a central collection point and accepts connections from the
agent. Reports are read in, tokenized and written to a MySQL database.

A separate hosts table is maintained, a record is created for each host that
checks in with the collection point. Having a list of all hosts facilitates
reporting.

The server looks at the current and historic data in each host thread and will
send notification e-mails to a specified address based on three criterion:

* System load exceeds threshold
* Swap utilization exceeds threshold
* Disk utilization on any reported partition exceeds threshold

A separate dashboard writtein in Python iterates through the host table and
for each host, prints the most recent available report in tabular format.

The following SQL will build the reports table:

```
CREATE TABLE reports (timestamp bigint, hostname varchar(68), numcpus varchar(8),
  physmem varchar(16), loadone varchar(12), loadfive varchar(12),
  loadfifteen varchar(12), swapused varchar(12), kernelver varchar(65),
  uptime varchar(16), diskreport varchar(68));
```

The following SQL will build the hosts table:

```
CREATE TABLE hosts (host varchar(258), hostid integer NOT NULL AUTO_INCREMENT
  PRIMARY KEY);
```

To install the host monitor on the server, configure a database, create a directory to host the configuration file, tune the configuration file as desired. For now, we can start the server interactively with a command like:

```
nohup ./hostmon_server -b addr -f /path/to/config.conf 2>&1 > /var/log/hostmon.log &
```

Or from /etc/rc.local using simply:

```
/path/to/hostmon_server -b addr -f /path/to/config.conf 2>&1 > /var/log/hostmon.log &
```

On each client, edit cron and insert a line similar to the following:

```
0,10,20,30,40,50       *       *       *       *       /path/to/hostmon_agent -h addr
```

The frequency can be set at any value, of course, excessively frequent collection will result in a large amount of data!

