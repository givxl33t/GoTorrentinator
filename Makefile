test-nasa-magnet:
	go run main.go -source $$(cat __torrentfiles/nasa.magnet) -out ./downloads

test-debian-magnet:
	go run main.go -source $$(cat __torrentfiles/debian.magnet) -out ./downloads

test-debian-file:
	go run main.go -source __torrentfiles/debian.torrent -out ./downloads