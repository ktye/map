package maps

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
)

// Tile is a 256x256 part of a map as an image in web Mercator projection (EPSG:3857).
//
// Zoom levels: 0-24
//	0: single tile of hole world
//	z: 2^z times 2^z tiles
//	16: should be good enough for not getting lost
//	19: max zoom level of tile.openstreetmap.org, some servers offer less.
//	24: full resolution if the data is stored as 2x32bit integer
// X coordinate:
//	from 0 (left edge 180 deg W) to 2^z - 1 (right edge is 180 E)
// Y coordinate:
//	from 0 (top edge is 85.0511 deg N) to 2^zoom - 1 (bottom edge is 85.0511 deg S)
//
// Reference:
// https://wiki.openstreetmap.org/wiki/Tiles
// https://wiki.openstreetmap.org/wiki/Slippy_map_tilenames
type Tile image.Image

// NumTiles returns the number of tiles per direction for the given zoom value.
// It returns 2^z for z values in the allowed range [0, 24] and 0 otherwise.
func NumTiles(z int) int {
	if z < 0 || z > 24 {
		return 0
	}
	return int(1 << uint(z))
}

// A TileServer can return a Tile.
//
// Example:
//	tileServer := CombinedTileServer{
//		CacheTileServer: NewCacheTileServer(10000),
//		LocalTileServer: "path/to/static/tiles",
//		HttpTileServer: "http://a.tileserver.mymap.com",
//	}
type TileServer interface {
	Get(z, x, y int) (Tile, error)
}

// HttpTileServer is a TileServer which requests tiles from a URL.
// It's value is the server base URL, e.g: "http://a.tileserver.mymap.com".
type HttpTileServer string

// Get returns the tile from HttpTileServer/z/x/y.png
func (s HttpTileServer) Get(z, x, y int) (Tile, error) {
	x, y = normalizeTile(z, x, y)

	u, err := url.Parse(string(s))
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, strconv.Itoa(z), strconv.Itoa(x), strconv.Itoa(y)+".png")
	url := u.String()

	log.Print("GET ", url)
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tile server response is not ok:%d: %s", res.StatusCode, res.Status)
	}
	if tile, err := decodePngTile(res.Body); err != nil {
		return nil, fmt.Errorf("tile server did not return a valid png: %s", err)
	} else {
		return tile, nil
	}
}

// LocalTileServer is the base directory for a static tile file system on disk.
type LocalTileServer string

// Get returns the tile from disk from the path LocalTile/z/x/y.png
func (l LocalTileServer) Get(z, x, y int) (Tile, error) {
	x, y = normalizeTile(z, x, y)
	file := filepath.Join(string(l), strconv.Itoa(z), strconv.Itoa(x), strconv.Itoa(y)+".png")
	if r, err := os.Open(file); err != nil {
		return nil, err
	} else {
		defer r.Close()
		return decodePngTile(r)
	}
}

// Add writes the tile to disk.
// It overwrites any existing file.
func (l LocalTileServer) Add(z, x, y int, t Tile) error {
	x, y = normalizeTile(z, x, y)
	if string(l) == "" {
		return errors.New("the local tile server path is unset")
	}
	dir := filepath.Join(string(l), strconv.Itoa(z), strconv.Itoa(x))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file := filepath.Join(dir, strconv.Itoa(y)+".png")
	if f, err := os.Create(file); err != nil {
		return err
	} else {
		defer f.Close()
		return png.Encode(f, t)
	}
}

// decodePngTile returns a Tile from a png read from r.
func decodePngTile(r io.Reader) (Tile, error) {
	if img, err := png.Decode(r); err != nil {
		return nil, err
	} else {
		if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
			return nil, errors.New("png tile size is not 256x256")
		}
		return Tile(img), nil
	}
}

// CacheTileServer is an in-memory TileServer.
// Use NewCacheTileServer to create and enable a CacheTileServer.
type CacheTileServer struct {
	maxTiles int // If this is non-zero, it does not store more tiles that this number.
	m        map[[3]int]Tile
	sync.Mutex
}

// Get returns a tile from the cache.
func (c *CacheTileServer) Get(z, x, y int) (Tile, error) {
	x, y = normalizeTile(z, x, y)
	c.Lock()
	defer c.Unlock()
	if t, ok := c.m[[3]int{z, x, y}]; !ok {
		return nil, errors.New("tile is not cached")
	} else {
		return t, nil
	}
}

// Add adds a tile to the cache.
// It returns immediately, if the CacheTileServer is not enabled.
func (c *CacheTileServer) Add(z, x, y int, t Tile) {
	x, y = normalizeTile(z, x, y)
	if c.m == nil {
		return
	}
	c.Lock()
	if c.maxTiles == 0 || len(c.m) < c.maxTiles {
		c.m[[3]int{z, x, y}] = t
	}
	c.Unlock()
}

// NewCacheTileServer enables and returns a CacheTileServer.
// Set maxTiles to 0 if there is no limit on the number of tiles to be cached.
func NewCacheTileServer(maxTiles int) *CacheTileServer {
	var c CacheTileServer
	c.m = make(map[[3]int]Tile)
	c.maxTiles = maxTiles
	return &c
}

// UniformTileServer returns tiles with a uniform color.
type UniformTileServer struct {
	Color color.Color
	im    *image.RGBA
}

// Get returns the color of u.
func (u *UniformTileServer) Get(z, x, y int) (Tile, error) {
	x, y = normalizeTile(z, x, y)
	if u.im == nil {
		u.im = image.NewRGBA(image.Rect(0, 0, 256, 256))
		draw.Draw(u.im, u.im.Bounds(), &image.Uniform{u.Color}, image.ZP, draw.Src)
	}
	return Tile(u.im), nil
}

// BlackTileServer always returns a black tile.
var BlackTileServer = UniformTileServer{Color: color.Black}

// A PointTileServer renders coordinates as points on a transparent background.
type PointTileServer struct {
	Color  color.Color
	File   string
	coords []LatLon
}

func NewPointTileServer(file string, c color.Color) (*PointTileServer) {
	var p PointTileServer
	if c == nil {
		c = color.Black
	}
	p.Color = c
	if f, err := os.Open(file); err != nil {
		panic(err)
	} else {
		defer f.Close()
		var lat, lon float64
		for {
			if n, err := fmt.Fscanf(f, "%f %f\n", &lat, &lon); n == 2 && err == nil {
				p.coords = append(p.coords, LatLon{Degree(lat), Degree(lon)})
			} else {
				break
			}
		}
	}
	return &p
}

func (p *PointTileServer) Get(z, x, y int) (Tile, error) {
	im := image.NewAlpha(image.Rect(0, 0, 256, 256))
	for _, c := range p.coords {
		if xy, err := c.XY(z); err != nil {
			if xy.X == x && xy.Y == y {
				im.Set(xy.XP, xy.YP, color.Opaque)
			}
		}
	}
	return Tile(im), nil
}

// CombinedTileServer combines an CachedTileServer a LocalTileServer and an HttpTileServer.
type CombinedTileServer struct {
	Points *PointTileServer
	Cache  *CacheTileServer
	Local  LocalTileServer
	Http   HttpTileServer
}

// Get returns a tile from the cache, the local filestystem or the net in that order.
// It skipps any mode if it is not configured.
// Any tiles retrieved tiles are also cached in the local and the cache tile server,
// if these are configured.
// Get never returns an error, if no tiles are present, it returns a black tile instead.
func (c CombinedTileServer) Get(z, x, y int) (Tile, error) {
	t, err := c.get(z, x, y)
	if err != nil {
		return t, err
	}
	if c.Points == nil {
		return t, nil
	}

	im := t.(draw.Image)
	for _, coords := range c.Points.coords {
		if xy, err := coords.XY(z); err == nil {
			if xy.X == x && xy.Y == y {
				im.Set(xy.XP, xy.YP, c.Points.Color)
			}
		}
	}
	return Tile(im), nil
}
func (c CombinedTileServer) get(z, x, y int) (Tile, error) {
	x, y = normalizeTile(z, x, y)
	if c.Cache.m != nil {
		if t, err := c.Cache.Get(z, x, y); err == nil {
			return t, nil
		}
	}
	if c.Local != LocalTileServer("") {
		if t, err := c.Local.Get(z, x, y); err == nil {
			if c.Cache.m != nil {
				c.Cache.Add(z, x, y, t)
			}
			return t, nil
		}
	}
	if c.Http != HttpTileServer("") {
		if t, err := c.Http.Get(z, x, y); err == nil {
			if c.Local != LocalTileServer("") {
				c.Local.Add(z, x, y, t)
			}
			if c.Cache.m != nil {
				c.Cache.Add(z, x, y, t)
			}
			return t, nil
		} else {
			log.Print(err)
		}
	}
	return BlackTileServer.Get(z, x, y)
}

// normalizeTile wraps tile coordinates around, if the x or y coordinates
// are out of range.
// Wrapping the x coordinate seems natural, as the definition of 0 is arbitrary.
// Instead of wrapping the y coordinate, an invalid (or black) tile could be send.
func normalizeTile(z, x, y int) (X, Y int) {
	checkZoom(z)
	m := NumTiles(z)
	x %= m
	y %= m
	if x < 0 {
		x += m
	}
	if y < 0 {
		y += m
	}
	return x, y
}
