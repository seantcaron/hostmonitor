#!/usr/bin/python

# Pull data from the Host Mon database and generate the Web dashboard
#  Sean Caron (scaron@umich.edu)

import cgi, time, sys, MySQLdb

print('Content-type: text/html\n')
print('<html>')
print('<head>')
print('<title>Host Mon</title>')
print('<meta http-equiv="refresh" content="600">')
print('<style type="text/css">* { border-radius: 5px; } h1 { font-family: Arial, Helvetica; } p { font-size: small; font-weight: bold; font-family: Arial, Helvetica; width: 80%; margin: 10px auto; } table { height: 15%; margin: 10px auto; width: 80%; } th { font-family: Arial, Helvetica; } td { font-family: Courier; }</style>')
print('</head>')
print('<body bgcolor=White text=Black vlink=Black text=Black>')
print('<h1>Host Mon: ' + time.strftime("%A %b %d %H:%M:%S %Z", time.localtime()) + '</h1>')

print('<table>')
print('<tr><th>Host name</th><th>Cores</th><th>Physmem (kB)</th><th>Load 1</th><th>Load 5</th><th>Load 15</th><th>Swap used (%)</th><th>Disk report (%util)</th></tr>')

db = MySQLdb.connect(user="hostmon",passwd="xyzzy123",db="hostmonitor")

curs = db.cursor()

query = 'SELECT host FROM hosts ORDER BY host ASC;'
curs.execute(query)
hosts = curs.fetchall()

toggle = 0

tcores = 0
tphysmem = 0
thosts = 0

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
        print('</td>')

	if float(row[4]) > float(row[2]):
	    print('<td bgcolor=#ffb3b3>')
	elif float(row[4]) > float(row[2])/2.0:
	    print('<td bgcolor=#ffffb3>')
        else:
	    print('<td>')

        print(row[4])
	print('</td>')

        if float(row[5]) > float(row[2]):
	    print('<td bgcolor=#ffb3b3>')
        elif float(row[5]) > float(row[2])/2.0:
	    print('<td bgcolor=#ffffb3>')
	else:
	    print('<td>')

	print(row[5])
	print('</td>')

        if float(row[6]) > float(row[2]):
	    print('<td bgcolor=#ffb3b3>')
        elif float(row[6]) > float(row[2])/2.0:
	    print('<td bgcolor=#ffffb3>')
	else:
	    print('<td>')

	print(row[6])
	print('</td>')

	if float(row[7]) > 66.0:
	    print('<td bgcolor=#ffb3b3>')
	elif float(row[7]) > 10.0:
	    print('<td bgcolor=#ffffb3>')
	else:
	    print('<td>')

	print(row[7])
	print('</td><td>')
	print(row[8])
        print('</td></tr>')

        tcores = tcores + int(row[2])
        tphysmem = tphysmem + int(row[3])
        thosts = thosts + 1

    toggle = not toggle

print('</table>')
print('<p>' + str(thosts) + ' total hosts, ' + str(tcores) + ' total cores, ' + str(tphysmem) + ' kB total physical memory</p>')
print('</body>')
print('</html>')

db.close()