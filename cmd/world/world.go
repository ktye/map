// World draws pixels from GPS coordinates to tiles.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"strconv"

	"github.com/ktye/map/tile"
)

func main() {
	var w world

	var ts, color string
	var update bool
	flag.StringVar(&ts, "tiles", "tiles", "directory for local tile server")
	flag.IntVar(&w.Zoom, "zoom", 11, "zoom level")
	flag.StringVar(&color, "color", "#FF0000", "color #RRGGBB")
	flag.BoolVar(&update, "update", false, "print a list with updated tiles")
	flag.Parse()

	w.Server = tile.LocalServer(ts)
	w.Color = parseColor(color)
	updatedTiles := make(map[string]bool)

	var lat, lon float64
	for {
		if n, err := fmt.Scanf("%f %f\n", &lat, &lon); n == 2 && err == nil {
			ll := tile.LatLon{tile.Degree(lat), tile.Degree(lon)}
			if xy, err := ll.XY(w.Zoom); err != nil {
				log.Fatal(err)
			} else {
				if err := w.addPoint(xy); err != nil {
					log.Fatal(err)
				}
				if update {
					updatedTiles[fmt.Sprintf("%d/%d/%d.png", xy.Z, xy.X, xy.Y)] = true
				}
			}
		} else {
			break
		}
	}
	w.flush()

	if update {
		for s := range updatedTiles {
			fmt.Println(s)
		}
	}
}

func parseColor(s string) color.RGBA {
	errmsg := "wrong color format, expecting #RRGGBBAA"
	if len(s) != 7 || s[0] != '#' {
		log.Fatal(errmsg)
	}
	s = s[1:]
	var v [3]uint8
	for i := 0; i < 3; i++ {
		if n, err := strconv.ParseUint(s[2*i:2*i+2], 16, 8); err != nil {
			log.Fatal(errmsg)
		} else {
			v[i] = uint8(n)
		}
	}
	return color.RGBA{R: v[0], G: v[1], B: v[2], A: 255}
}

type world struct {
	Points  string
	Zoom    int
	Color   color.Color
	Server  tile.LocalServer
	x, y    int
	current tile.Tile
}

func (w *world) addPoint(xy tile.XY) error {
	if xy.X != w.x || xy.Y != w.y {
		if w.current != nil {
			if err := w.Server.Add(w.Zoom, w.x, w.y, w.current); err != nil {
				return err
			}
		}
		if t, err := w.Server.Get(w.Zoom, xy.X, xy.Y); err != nil {
			im := image.NewRGBA(image.Rect(0, 0, 256, 256))
			w.current = im
		} else {
			w.current = t
		}
		w.x = xy.X
		w.y = xy.Y
	}
	w.current.Set(xy.XP, xy.YP, w.Color)
	return nil
}

func (w world) flush() error {
	if w.current != nil {
		if err := w.Server.Add(w.Zoom, w.x, w.y, w.current); err != nil {
			return err
		}
	}
	return nil
}
