package maps

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
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

// A TileServer can return a Tile.
//
// Example:
//	tileServer := CombinedTileServer{
//		CacheTileServer: NewCacheTileServer(10000),
//		LocalTileServer: "path/to/static/tiles",
//		HttpTileServer: "http://a.tile.openstreetmap.org",
//	}
type TileServer interface {
	Get(z, x, y int) (Tile, error)
}

// HttpTileServer is a TileServer which requests tiles from a URL.
// It's value is the server base URL, e.g: "http://a.tile.openstreetmap.org"
// E.g.: "http://a.tile.openstreetmap.org".
type HttpTileServer string

// Get returns the tile from HttpTileServer/z/x/y.png
func (s HttpTileServer) Get(z, x, y int) (Tile, error) {
	url := path.Join(string(s), strconv.Itoa(z), strconv.Itoa(x), strconv.Itoa(y)+".png")
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
	if u.im == nil {
		u.im = image.NewRGBA(image.Rect(0, 0, 256, 256))
		draw.Draw(u.im, u.im.Bounds(), &image.Uniform{u.Color}, image.ZP, draw.Src)
	}
	return Tile(u.im), nil
}

// BlackTileServer always returns a black tile.
var BlackTileServer = UniformTileServer{Color: color.Black}

// CombinedTileServer combines an CachedTileServer a LocalTileServer and an HttpTileServer.
type CombinedTileServer struct {
	Cache *CacheTileServer
	Local LocalTileServer
	Http  HttpTileServer
}

// Get returns a tile from the cache, the local filestystem or the net in that order.
// It skipps any mode if it is not configured.
// Any tiles retrieved tiles are also cached in the local and the cache tile server,
// if these are configured.
// Get never returns an error, if no tiles are present, it returns a black tile instead.
func (c CombinedTileServer) Get(z, x, y int) (Tile, error) {
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
		}
	}
	return BlackTileServer.Get(z, x, y)
}
