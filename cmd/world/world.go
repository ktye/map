// World creates a map for OruxMaps with points from a list.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ktye/map/orux"
	"github.com/ktye/map/tile"
)

// TODO: it does not work: no output
// TODO: cache tiles on the file system with the ability to update these for a short list of new points.
// TODO: multiple zoom levels, tile.SparsePointServer needs to be modified for orux.Encode

type world struct {
	Name   string
	Zoom   int
	File   string
	points []tile.LatLon
}

func main() {
	var w world
	flag.StringVar(&w.Name, "name", "MyWorld", "directory name for output orux map")
	flag.IntVar(&w.Zoom, "zoom", 2, "zoom level")
	flag.StringVar(&w.File, "file", "points.txt", "lat-lon list of coordinates")
	flag.Parse()

	w.read()

	var ts tile.Server
	if s, err := tile.NewSparsePointServer(w.Zoom, w.points); err != nil {
		log.Fatal(err)
	} else {
		ts = s
	}

	m := orux.Map{
		TopLeft:     tile.LatLon{tile.MaxLatitude, -180},
		BottomRight: tile.LatLon{tile.MinLatitude(w.Zoom), 180},
		ZoomLevels:  []int{w.Zoom},
	}
	if err := m.Encode(w.Name, ts); err != nil {
		log.Fatal(err)
	}
}

func (w *world) read() {
	if f, err := os.Open(w.File); err != nil {
		log.Fatal(err)
	} else {
		defer f.Close()
		var lat, lon float64
		for {
			if n, err := fmt.Fscanf(f, "%f %f\n", &lat, &lon); n == 2 && err == nil {
				w.points = append(w.points, tile.LatLon{tile.Degree(lat), tile.Degree(lon)})
			} else {
				break
			}
		}
	}
}
