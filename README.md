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

At some point, a process will run reports against the database, which can be
used to generate notification e-mails, prepare a Web dashboard and so on.

