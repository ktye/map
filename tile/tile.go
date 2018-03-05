package tile

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
type Tile draw.Image

// Server can return a Tile.
//
// Example:
//	tileServer := CombinedServer{
//		CacheServer: NewCacheServer(10000),
//		LocalServer: "path/to/static/tiles",
//		HttpServer: "http://a.tileserver.mymap.com",
//	}
type Server interface {
	Get(z, x, y int) (Tile, error)
}

// SparseServer may not contain all tiles.
// Get returns the black tile, if it is not available.
// Next iterates over all available tiles and returns a nil error, if the tile is valid.
type SparseServer interface {
	Get(z, x, y int) (Tile, error)
	Next() (int, int, int, Tile, error) // z, x, y, tile, error
}

// HttpServer is a Server which requests tiles from a URL.
// It's value is the server base URL, e.g: "http://a.tileserver.mymap.com".
type HttpServer string

// Get returns the tile from HttpServer/z/x/y.png
func (s HttpServer) Get(z, x, y int) (Tile, error) {
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

// LocalServer is the base directory for a static tile file system on disk.
type LocalServer string

// Get returns the tile from disk from the path LocalTile/z/x/y.png
func (l LocalServer) Get(z, x, y int) (Tile, error) {
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
func (l LocalServer) Add(z, x, y int, t Tile) error {
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
		return Tile(img.(draw.Image)), nil
	}
}

// CacheServer is an in-memory Server.
// Use NewCacheServer to create and enable a CacheServer.
type CacheServer struct {
	maxTiles int // If this is non-zero, it does not store more tiles that this number.
	m        map[[3]int]Tile
	sync.Mutex
}

// Get returns a tile from the cache.
func (c *CacheServer) Get(z, x, y int) (Tile, error) {
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
// It returns immediately, if the CacheServer is not enabled.
func (c *CacheServer) Add(z, x, y int, t Tile) {
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

// NewCacheServer enables and returns a CacheServer.
// Set maxTiles to 0 if there is no limit on the number of tiles to be cached.
func NewCacheServer(maxTiles int) *CacheServer {
	var c CacheServer
	c.m = make(map[[3]int]Tile)
	c.maxTiles = maxTiles
	return &c
}

// UniformServer returns tiles with a uniform color.
type UniformServer struct {
	Color color.Color
	im    *image.RGBA
}

// Get returns the color of u.
func (u *UniformServer) Get(z, x, y int) (Tile, error) {
	x, y = normalizeTile(z, x, y)
	if u.im == nil {
		u.im = image.NewRGBA(image.Rect(0, 0, 256, 256))
		draw.Draw(u.im, u.im.Bounds(), &image.Uniform{u.Color}, image.ZP, draw.Src)
	}
	return Tile(u.im), nil
}

// A PointServer renders coordinates as points on a transparent background.
type PointServer struct {
	Color  color.Color
	File   string
	coords []LatLon
}

func NewPointServer(file string, c color.Color) *PointServer {
	var p PointServer
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

func (p *PointServer) Get(z, x, y int) (Tile, error) {
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

// NewSparserPointServer returns a SparseServer for a list of points and a given zoom level.
func NewSparsePointServer(z int, points []LatLon) (*SparsePointServer, error) {
	if z < 0 || z > 24 {
		return nil, ZoomRangeError
	}
	s := SparsePointServer{
		z:      z,
		points: make(map[point][]point),
	}
	var xy XY
	var key point
	var err error
	for _, ll := range points {
		xy, err = ll.XY(z)
		if err != nil {
			return nil, err
		}
		key.x = xy.X
		key.y = xy.Y
		pts, ok := s.points[key]
		pts = append(pts, point{xy.XP, xy.YP})
		s.points[key] = pts
		if ok == false {
			s.keys = append(s.keys, key)
		}
	}
	return &s, nil
}

// SparsePointServer implements a PointServer as a SparseServer.
type SparsePointServer struct {
	z      int
	points map[point][]point // Keys are the tile indexes {tile.X, tile.Y}, value is a slices of pixel indexes {tile.XP, tile.YP}.
	keys   []point           // index list for the points map
	p      int
}

func (s *SparsePointServer) Get(z, x, y int) (Tile, error) {
	if z != s.z {
		return nil, fmt.Errorf("SparsePointServer: Get called with zoom level %d, but only %d is available", z, s.z)
	}
	im := image.NewAlpha(image.Rect(0, 0, 256, 256))
	if points, ok := s.points[point{x, y}]; ok {
		for _, pt := range points {
			im.Set(pt.x, pt.y, color.Opaque)
		}
	}
	return im, nil
}

func (s *SparsePointServer) Next() (z, x, y int, t Tile, err error) {
	if s.p >= len(s.keys) {
		return 0, 0, 0, nil, io.EOF
	}
	key := s.keys[s.p]
	s.p++
	t, err = s.Get(z, x, y)
	return s.z, key.x, key.y, t, err
}

type point struct {
	x, y int
}

// CombinedServer combines an CachedServer a LocalServer and an HttpServer.
type CombinedServer struct {
	Points *PointServer
	Cache  *CacheServer
	Local  LocalServer
	Http   HttpServer
}

// Get returns a tile from the cache, the local filesystem or the net in that order.
// It skipps any mode if it is not configured.
// Any tiles retrieved are also cached in the local and the cache tile server,
// if these are configured.
// Get never returns an error, if no tiles are present, it returns a black tile instead.
func (c CombinedServer) Get(z, x, y int) (Tile, error) {
	t, err := c.get(z, x, y)
	if err != nil {
		return t, err
	}
	if c.Points == nil {
		return t, nil
	}

	for _, coords := range c.Points.coords {
		if xy, err := coords.XY(z); err == nil {
			if xy.X == x && xy.Y == y {
				t.Set(xy.XP, xy.YP, c.Points.Color)
			}
		}
	}
	return t, nil
}
func (c CombinedServer) get(z, x, y int) (Tile, error) {
	x, y = normalizeTile(z, x, y)
	if c.Cache != nil && c.Cache.m != nil {
		if t, err := c.Cache.Get(z, x, y); err == nil {
			return t, nil
		}
	}
	if c.Local != LocalServer("") {
		if t, err := c.Local.Get(z, x, y); err == nil {
			if c.Cache != nil && c.Cache.m != nil {
				c.Cache.Add(z, x, y, t)
			}
			return t, nil
		}
	}
	if c.Http != HttpServer("") {
		if t, err := c.Http.Get(z, x, y); err == nil {
			if c.Local != LocalServer("") {
				c.Local.Add(z, x, y, t)
			}
			if c.Cache != nil && c.Cache.m != nil {
				c.Cache.Add(z, x, y, t)
			}
			return t, nil
		} else {
			log.Print(err)
		}
	}
	return black, nil
}

// NumTiles returns the number of tiles per direction for the given zoom value.
// It returns 2^z for z values in the allowed range [0, 24] and 0 otherwise.
func NumTiles(z int) int {
	if z < 0 || z > 24 {
		return 0
	}
	return int(1 << uint(z))
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

var black Tile

func init() {
	im := image.NewRGBA(image.Rect(0, 0, 256, 256))
	draw.Draw(im, im.Bounds(), &image.Uniform{color.Black}, image.ZP, draw.Src)
	black = im
}
