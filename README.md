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

The server looks at the current and historic data in each host thread and will
send notification e-mails to a specified address based on three criterion:

* System load exceeds threshold
* Swap utilization exceeds threshold
* Disk utilization on any reported partition exceeds threshold

A separate app will be implemented to, for example, prepare a Web dashboard from
the contents of the database.

The following SQL will build the host table:

```
CREATE TABLE reports (timestamp bigint, hostname varchar(68), numcpus varchar(8),
  physmem varchar(16), loadone varchar(12), loadfive varchar(12),
  loadfifteen varchar(12), swapused varchar(12), diskreport varchar(68));
```
