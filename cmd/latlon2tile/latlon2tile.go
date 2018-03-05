// Latlon2tile converts coordinates to tiles.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/ktye/map/tile"
)

func main() {
	var zoom int
	var lat, lon float64
	flag.Float64Var(&lat, "lat", 50.0, "latitude (degree)")
	flag.Float64Var(&lon, "lon", 0.0, "longitude (degree)")
	flag.IntVar(&zoom, "zoom", 11, "zoom level")
	flag.Parse()

	ll := tile.LatLon{tile.Degree(lat), tile.Degree(lon)}
	if xy, err := ll.XY(zoom); err != nil {
		log.Fatal(err)
	} else {
		fmt.Println(xy)
	}
}
