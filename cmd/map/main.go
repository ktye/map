// Derived from golang.org/x/exp/shiny/examples/tile.

// Program map shows a slippery map served by a tile.Server in a shiny frontend.
package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"log"
	"sync"

	"github.com/ktye/map/tile"
	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
)

var (
	generation int

	tileSize   = image.Point{256, 256}
	tileBounds = image.Rectangle{Max: tileSize}
)

var Origin = image.Point{}
var Zoom int
var tileServer tile.Server

func main() {
	// Process command line arguments.
	var cache int
	var local, url, points string
	flag.IntVar(&cache, "cache", 10000, "max number of cached files, set to -1 to disable completely")
	flag.StringVar(&local, "local", "", "directory of local file server, disabled by default")
	flag.StringVar(&url, "url", "", "URL of a http tile server")
	flag.IntVar(&Zoom, "zoom", 0, "zoom level [0..24]")
	flag.StringVar(&points, "points", "points.dat", "file name of points file")
	flag.Parse()

	if Zoom < 0 || Zoom > 24 {
		log.Fatal(tile.ZoomRangeError)
	}

	// Start the tile server.
	if url == "" && local == "" {
		tileServer = tile.Mandelbrot{}
	} else {
		tileServer = tile.CombinedServer{
			Points: tile.NewPointServer(points, color.RGBA{0, 255, 0, 255}),
			Cache:  tile.NewCacheServer(cache),
			Local:  tile.LocalServer(local),
			Http:   tile.HttpServer(url),
		}
	}

	driver.Main(func(s screen.Screen) {
		w, err := s.NewWindow(nil)
		if err != nil {
			log.Fatal(err)
		}
		defer w.Release()

		var (
			pool = &tilePool{
				screen:   s,
				drawRGBA: drawRGBA,
				m:        map[image.Point]*tilePoolEntry{},
			}
			dragging     bool
			paintPending bool
			drag         image.Point
			sz           size.Event
		)
		for {
			switch e := w.NextEvent().(type) {
			case lifecycle.Event:
				if e.To == lifecycle.StageDead {
					return
				}

			case key.Event:
				if e.Code == key.CodeEscape {
					return
				}

			case mouse.Event:
				p := image.Point{X: int(e.X), Y: int(e.Y)}

				if e.Button == mouse.ButtonLeft && e.Direction != mouse.DirNone {
					dragging = e.Direction == mouse.DirPress
					drag = p
				}

				// ScrollWheel: zoom in/out.
				// Keep the location of the point under the cursor.
				if e.Button == mouse.ButtonWheelUp || e.Button == mouse.ButtonWheelDown {
					if e.Button == mouse.ButtonWheelUp && Zoom < 24 {
						Zoom++
						Origin = Origin.Mul(2)
						Origin = Origin.Add(p) // TODO: p is absolute, should be relative to the window.
					} else if e.Button == mouse.ButtonWheelDown && Zoom > 0 {
						Zoom--
						Origin = Origin.Sub(p) // TODO: same as above.
						Origin = Origin.Div(2)
					} else {
						break
					}
					if !paintPending {
						paintPending = true
						pool.reset()
						w.Send(paint.Event{})
					}
					break
				}

				if !dragging {
					break
				}
				Origin = Origin.Sub(p.Sub(drag))
				drag = p
				if Origin.X < 0 {
					Origin.X = 0
				}
				if Origin.Y < 0 {
					Origin.Y = 0
				}
				if !paintPending {
					paintPending = true
					w.Send(paint.Event{})
				}

			case paint.Event:
				generation++
				var wg sync.WaitGroup
				for y := -(Origin.Y & 0xff); y < sz.HeightPx; y += 256 {
					for x := -(Origin.X & 0xff); x < sz.WidthPx; x += 256 {
						wg.Add(1)
						go drawTile(&wg, w, pool, Origin, x, y)
					}
				}
				wg.Wait()
				w.Publish()
				paintPending = false
				pool.releaseUnused()

			case size.Event:
				sz = e

			case error:
				log.Print(e)
			}
		}
	})
}

func drawTile(wg *sync.WaitGroup, w screen.Window, pool *tilePool, Origin image.Point, x, y int) {
	defer wg.Done()
	tp := image.Point{
		(x + Origin.X) >> 8,
		(y + Origin.Y) >> 8,
	}
	tex, err := pool.get(tp)
	if err != nil {
		log.Println(err)
		return
	}
	w.Copy(image.Point{x, y}, tex, tileBounds, screen.Src, nil)
}

func drawRGBA(m *image.RGBA, tp image.Point) {
	srcImg, err := tileServer.Get(Zoom, tp.X, tp.Y)
	if err != nil {
		log.Print(err)
		srcImg = image.White
	}
	draw.Draw(m, m.Bounds(), srcImg, image.Point{}, draw.Src)
}

type tilePoolEntry struct {
	tex screen.Texture
	gen int
}

type tilePool struct {
	screen   screen.Screen
	drawRGBA func(*image.RGBA, image.Point)

	mu sync.Mutex
	m  map[image.Point]*tilePoolEntry
}

func (p *tilePool) get(tp image.Point) (screen.Texture, error) {
	p.mu.Lock()
	v, ok := p.m[tp]
	if v != nil {
		v.gen = generation
	}
	p.mu.Unlock()

	if ok {
		return v.tex, nil
	}
	tex, err := p.screen.NewTexture(tileSize)
	if err != nil {
		return nil, err
	}
	buf, err := p.screen.NewBuffer(tileSize)
	if err != nil {
		tex.Release()
		return nil, err
	}
	p.drawRGBA(buf.RGBA(), tp)
	tex.Upload(image.Point{}, buf, tileBounds)
	buf.Release()

	p.mu.Lock()
	p.m[tp] = &tilePoolEntry{
		tex: tex,
		gen: generation,
	}
	p.mu.Unlock()

	return tex, nil
}

func (p *tilePool) reset() {
	p.mu.Lock()
	for tp, v := range p.m {
		v.gen--
		p.m[tp] = v
	}
	p.mu.Unlock()
	p.releaseUnused()
}

func (p *tilePool) releaseUnused() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for tp, v := range p.m {
		if v.gen == generation {
			continue
		}
		v.tex.Release()
		delete(p.m, tp)
	}
}
