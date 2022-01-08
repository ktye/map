package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/golang/freetype/raster"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"m4o.io/pbf"
	"math"
	"os"
)

const Lat, Lon float64 = 50.10663, 8.66227
const Zoom uint32 = 24 - 16

type LatLon [2]float64
type Point struct{ x, y uint32 }

func mercator(ll LatLon) Point {
	const s = float64(math.MaxUint32 / 2)
	la := ll[0] * math.Pi / 180.0
	x := uint32(math.MinInt32 + s*ll[1]/180.0)
	y := uint32(s * (1 - math.Log(math.Tan(la)+1/math.Cos(la))/math.Pi))
	return Point{x, y}
}

func scale(z uint32, ref Point, p Point) (uint32, uint32) {
	return (p.x - ref.x) >> z, (p.y - ref.y) >> z
}

type rgb uint32

func (c rgb) RGBA() (r, g, b, a uint32) {
	r = (uint32(c)&0xff0000)>>8 | 0xff
	g = (uint32(c) & 0x00ff00) | 0xff
	b = (uint32(c)&0x0000ff)<<8 | 0xff
	a = 0xffff
	return r, g, b, a
}

type Style struct {
	key, val string
	lw       int // 0: fill
	co       rgb
}

var Styles = []Style{
	{"building", "", 0, 0xa12725},
	{"natural", "water", 0, 0x456ecc},
	{"natural", "wood", 0, 0x4a6741},
	{"landuse", "forest", 0, 0x4a6741},
	{"landuse", "farmland", 0, 0x818c3c},
	{"landuse", "farmyard", 0, 0x818c3c},
	{"landuse", "meadow", 0, 0x818c3c},
	{"landuse", "orchard", 0, 0x818c3c},
	{"waterway", "grassland", 0, 0x818c3c},
	{"waterway", "stream", 2, 0x2558cf},
	{"highway", "residential", 3, 0x5f5f5f},
	{"highway", "service", 1, 0x5f5f5f},
	{"highway", "footway", 2, 0xb3b3b3},
	{"highway", "path", 2, 0x593a0e},
	{"highway", "", 3, 0x5f5f5f},
	{"tracktype", "", 2, 0x593a0e},
}

func match(tags map[string]string) (int, color.Color) {
	for k, v := range tags {
		for _, s := range Styles {
			if k == s.key && (v == s.val || (v != "" && s.val == "")) {
				return s.lw, s.co
			}
		}
	}
	fmt.Println(tags)
	return 1, color.Gray{128}
}

func main() {
	file, e := os.Open("h.osm.pbf")
	fatal(e)
	defer file.Close()

	top := mercator(LatLon{Lat, Lon})
	nodes := make(map[uint64]Point)

	d, e := pbf.NewDecoder(context.Background(), file)
	fatal(e)

	const W, H, Z uint32 = 1024, 1024, 24 - 16
	m := image.NewRGBA(image.Rect(0, 0, int(W), int(H)))
	draw.Draw(m, m.Bounds(), &image.Uniform{color.White}, image.ZP, draw.Src)
	painter := raster.NewRGBAPainter(m)
	rast := raster.NewRasterizer(m.Bounds().Max.X, m.Bounds().Max.Y)

	clip := func(v []Point) []Point {
		a := 0
		for i := range v {
			if v[i].x >= W || v[i].y >= H {
				if i == 0 || a == i {
					a = 1 + i
				} else {
					return v[a:i]
				}
			}
		}
		return v[a:]
	}
	makePath := func(v []Point) raster.Path {
		var path raster.Path
		for i, p := range v {
			x, y := fixed.Int26_6(p.x<<6), fixed.Int26_6(p.y<<6)
			if i == 0 {
				path = append(path, 0, x, y, 0)
			} else {
				path = append(path, 1, x, y, 1)
			}
		}
		return path
	}
	fillPath := func(v []Point, co color.Color) {
		if len(v) < 3 {
			return
		}
		painter.SetColor(co)
		rast.AddPath(makePath(v))
		rast.Rasterize(painter)
		rast.Clear()
	}
	drawLine := func(v []Point, lw int, co color.Color) {
		if len(v) < 2 {
			return
		}
		path := makePath(v)
		rast.UseNonZeroWinding = true
		rast.AddStroke(path, fixed.I(lw), raster.SquareCapper, raster.BevelJoiner)
		painter.SetColor(co)
		rast.Rasterize(painter)
		rast.Clear()
	}
	drawWay := func(v []uint64, t map[string]string) {
		p := make([]Point, len(v))
		in := false
		for i, u := range v {
			p[i].x, p[i].y = scale(Zoom, top, nodes[u])
			if p[i].x < W && p[i].y < H {
				in = true
			}
		}
		if in {
			lw, co := match(t)
			if lw == 0 {
				fillPath(clip(p), co)
			} else {
				drawLine(clip(p), lw, co)
			}
		}
	}
	for {
		p, e := d.Decode()
		if e == io.EOF {
			break
		}
		fatal(e)
		switch v := p.(type) {
		case *pbf.Node:
			nodes[v.ID] = mercator(LatLon{float64(v.Lat), float64(v.Lon)})
		case *pbf.Way:
			drawWay(v.NodeIDs, v.Tags)
		}
	}
	var buf bytes.Buffer
	fatal(png.Encode(&buf, m))
	fatal(os.WriteFile("out.png", buf.Bytes(), 0644))
}

func fatal(e error) {
	if e != nil {
		panic(e)
	}
}
