#!/usr/bin/python

# Pull data from the CWho database and generate the Web dashboard
#  Sean Caron (scaron@umich.edu)

import cgi, time, sys, MySQLdb

print('Content-type: text/html\n')
print('<html>')
print('<head>')
print('<title>Host Monitor</title>')
print('<style type="text/css">* { border-radius: 5px; } h1 { font-family: Arial, Helvetica; } p { font-size: small; font-weight: bold; font-family: Arial, Helvetica; width: 80%; margin: 10px auto; } table { height: 15%; margin: 10px auto; width: 80%; } th { font-family: Arial, Helvetica; } td { font-family: Courier; }</style>')
print('</head>')
print('<body bgcolor=White text=Black vlink=Black text=Black>')
print('<h1>Host Monitor: ' + time.strftime("%A %b %d %H:%m:%S %Z", time.localtime()) + '</h1>')

print('<table>')
print('<tr><th>Host name</th><th>Cores</th><th>Physmem (kB)</th><th>Load 1</th><th>Load 5</th><th>Load 15</th><th>Swap used (%)</th><th>Disk report (%util)</th></tr>')

db = MySQLdb.connect(user="hostmon",passwd="xyzzy123",db="hostmonitor")

curs = db.cursor()

query = 'SELECT host from hosts;'
curs.execute(query)
hosts = curs.fetchall()

toggle = 0

tcores = 0
tphysmem = 0

for host in hosts:
    query = 'SELECT * FROM reports WHERE hostname = \'' + host[0] + '\' ORDER BY timestamp DESC LIMIT 1;'

    curs.execute(query)

    report = curs.fetchall()

    for row in report:
        if toggle == 0:
            print('<tr bgcolor=#ccffcc><td>')
        else:
            print('<tr><td>')
    
        print(row[1])
        print('</td><td>')
        print(row[2])
        print('</td><td>')
        print(row[3])
        print('</td><td>')
        print(row[4])
	print('</td><td>')
	print(row[5])
	print('</td><td>')
	print(row[6])
	print('</td><td>')
	print(row[7])
	print('</td><td>')
	print(row[8])
        print('</td></tr>')

        tcores = tcores + int(row[2])
        tphysmem = tphysmem + int(row[3])

    toggle = not toggle

# We need to commit() the query on inserts and modifies after execution before they actually take effect
# db.commit()

print('</table>')
print('<p>Total cores ' + str(tcores) + ', total physical memory ' + str(tphysmem) + ' kB</p>')
print('</body>')
print('</html>')

db.close()
