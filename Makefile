test-run-magnet:
	go run main.go -source $$(cat __torrentfiles/nasa.magnet) -out ./downloads

test-run-file:
	go run main.go -source __torrentfiles/debian-12.8.0-amd64-netinst.iso.torrent -out ./downloads