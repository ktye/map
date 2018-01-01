# track and map
cartography

Map should become a cartography infrastructure for [go](https://www.golang.org).

# Status
- [x] `coordinates.go`: Spherical coordinates transformations
- [x] `tile.go`: Tile definitions and tile server interfaces
- [ ] maps: doc.go
- [ ] path overlays
- [ ] file imports: gpx, fit, ...
- [x] `cmd/map`: application of a slippery map
	- [ ] accepting file drops for path overlay
	- [ ] track summary
	- [ ] multi-stage support
	- [ ] altitude graphs
	- [ ] overlay all travelled points
	- [ ] answer the question: Have I been here before? When?
	- as a:
		- [x] desktop application with shiny frontend
		- [ ] web server application
		- [ ] mobile application
- [ ] .osm.pbf -> extract -> render -> tile server pipeline
- A long way to go

![](http://www.walter-kuhl.de/grafik_f/mfundeg/01_messpunkt6759.jpg)

