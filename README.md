Host Monitor
------------

Sean Caron
scaron@umich.edu

This is a simple client-server health monitor written in Go.

A small agent runs on the client and collects the following data
points:

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
send notification e-mails to a specified address ased on the following criterion:

* System load exceeds threshold
* Swap utilization exceeds threshold
* Disk utilization on any reported partition exceeds threshold

A separate app will be implemented to, for example, prepare a Web dashboard from
the contents of the database.

