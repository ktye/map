// Package orux encodes raster tiles in a format oruxmaps can read.
//
// It uses the sqlite3 external command to write the database file.
package orux

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/ktye/maps/tile"
)

var tileLimit = int64(40) // Encode will complain if this limit is exceeded.

// Map defines the rectangle of the map and the zoom levels to be stored.
// The rectangle will be extended to the tile boundaries for the lowest ZoomLevel containing From and To.
type Map struct {
	TopLeft, BottomRight tile.LatLon
	ZoomLevels           []int
}

// Encode creates a directory with the given Name and writes 2 files to the directory:
// The index file name.otrk2.xml and the database file OruxMapsImages.db.
// The image data is retrieved from the TileServer.
func (m Map) Encode(name string, ts tile.TileServer) error {
	// Refuse to write a file which is too big.
	if n, err := m.Count(); err != nil {
		return err
	} else if n > tileLimit {
		return fmt.Errorf("requested map is too large: %d tiles (>%d)", n, tileLimit)
	}

	if err := os.Mkdir(name, 0744); err != nil {
		return err
	}

	// Temporarily write sqlite3 db file.
	// Or can sqlite3 pipe the database to stdout?
	dbfile := filepath.Join(name, "OruxMapsImages.db")
	cmd := exec.Command("sqlite3", dbfile)
	if wc, err := cmd.StdinPipe(); err != nil {
		return err
	} else {
		go m.sqlitePipe(wc, ts)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s", err, out)
		}
	}

	// Write ${name}/${name}.otrk2.xml
	if err := m.WriteXML(name); err != nil {
		return err
	}
	return nil
}

// sqlitePipe creates the database file by writing commands to the
// sqlite3 process on wc.
func (m Map) sqlitePipe(wc io.WriteCloser, ts tile.TileServer) {
	defer wc.Close()
	wc.Write([]byte(sqlStart))
	var buf bytes.Buffer
	for _, z := range m.ZoomLevels {
		tl, _ := m.TopLeft.XY(z)
		br, _ := m.BottomRight.XY(z)
		for x := tl.X; x <= br.X; x++ {
			for y := tl.Y; y <= br.Y; y++ {
				if tile, err := ts.Get(z, x, y); tile != nil {
					buf.Reset()
					png.Encode(&buf, tile)
					fmt.Fprintf(wc, "INSERT INTO \"tiles\" VALUES(%d,%d,%d,X'%s');", x-tl.X, y-tl.Y, z, hex.EncodeToString(buf.Bytes()))
				} else {
					fmt.Println(err)
				}
			}
		}
	}
	wc.Write([]byte(sqlEnd))
}

// Count calculates the number of tiles inside the map.
func (m Map) Count() (int64, error) {
	var sum int64
	for _, z := range m.ZoomLevels {
		var tl, br tile.XY
		if xy, err := m.TopLeft.XY(z); err != nil {
			return 0, err
		} else {
			tl = xy
		}
		if xy, err := m.BottomRight.XY(z); err != nil {
			return 0, err
		} else {
			br = xy
		}
		if br.X < tl.X || br.Y < tl.Y {
			return 0, fmt.Errorf("wrong definition for top left or buttom right corners")
		}
		sum += int64(br.X-tl.X) * int64(br.Y-tl.Y)
	}
	return sum, nil
}

// WriteXML writes the map index to ${name}/${name}.otrk2.xml.
func (m Map) WriteXML(name string) error {
	t := template.Must(template.New("xml").Parse(xmlTemplate))
	type xmlLayer struct {
		Zoom           string
		Name           string
		Xmax, Ymax     string
		MinLat, MaxLat string
		MinLon, MaxLon string
	}
	type xmlMap struct {
		Name   string
		Layers []xmlLayer
	}
	xmlName := filepath.Join(name, name+".otrk2.xml")
	if f, err := os.Create(xmlName); err != nil {
		return err
	} else {
		defer f.Close()
		x := xmlMap{
			Name: name,
		}
		x.Layers = make([]xmlLayer, len(m.ZoomLevels))
		for i, z := range m.ZoomLevels {
			if tl, br, nx, ny, err := m.expandTileCorners(z); err != nil {
				return err
			} else {
				x.Layers[i].Name = fmt.Sprintf("%s %02d", name, z)
				x.Layers[i].Zoom = fmt.Sprintf("%d", z)
				x.Layers[i].Xmax = fmt.Sprintf("%d", nx)
				x.Layers[i].Ymax = fmt.Sprintf("%d", ny)
				x.Layers[i].MinLat = fmt.Sprintf("%f", br.Lat)
				x.Layers[i].MaxLat = fmt.Sprintf("%f", tl.Lat)
				x.Layers[i].MinLon = fmt.Sprintf("%f", tl.Lon)
				x.Layers[i].MaxLon = fmt.Sprintf("%f", br.Lon)
			}
		}
		return t.Execute(f, x)
	}
}

// expandTileCorners returns the topLeft and bottomRight coordinates of the tile corners
// for the given zoom level and the number of tiles in x and y direction.
func (m Map) expandTileCorners(zoom int) (tl tile.LatLon, br tile.LatLon, nx, ny int, err error) {
	var xy tile.XY
	if xy, err = m.TopLeft.XY(zoom); err != nil {
		return tl, br, 0, 0, err
	} else {
		xy.XP, xy.YP = 0, 0
		tl = xy.LatLon()
		nx, ny = xy.X, xy.Y
	}
	if xy, err = m.BottomRight.XY(zoom); err != nil {
		return tl, br, 0, 0, err
	} else {
		xy.XP, xy.YP = 255, 255
		br = xy.LatLon()
		nx = xy.X - nx + 1
		ny = xy.Y - ny + 1
	}
	return tl, br, nx, ny, nil
}

const sqlStart = `
PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE tiles (x int, y int, z int, image blob, PRIMARY KEY (x,y,z));
`

const sqlEnd = `CREATE INDEX IND on tiles (x,y,z);
COMMIT;
`

const xmlTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<OruxTracker xmlns="http://oruxtracker.com/app/res/calibration"
 versionCode="3.0">
	<MapCalibration layers="true" layerLevel="0">
		<MapName><![CDATA[{{.Name}}]]></MapName>
		{{range $i, $x := $.Layers}}
		<OruxTracker xmlns="http://oruxtracker.com/app/res/calibration"
			 versionCode="2.1">
			<MapCalibration layers="false" layerLevel="{{$x.Zoom}}">
				<MapName><![CDATA[{{$x.Name}}]]></MapName>
				<MapChunks xMax="{{$x.Xmax}}" yMax="{{$x.Ymax}}" datum="WGS84" projection="Mercator" img_height="256" img_width="256" file_name="{{$x.Name}}" />
				<MapDimensions height="256" width="256" />
				<MapBounds minLat="{{$x.MinLat}}" maxLat="{{$x.MaxLat}}" minLon="{{$x.MinLon}}" maxLon="{{$x.MinLon}}" />
				<CalibrationPoints>
					<CalibrationPoint corner="TL" lon="{{$x.MinLon}}" lat="{{$x.MaxLat}}" />
					<CalibrationPoint corner="BR" lon="{{$x.MaxLon}}" lat="{{$x.MinLat}}" />
					<CalibrationPoint corner="TR" lon="{{$x.MaxLon}}" lat="{{$x.MaxLat}}" />
					<CalibrationPoint corner="BL" lon="{{$x.MinLon}}" lat="{{$x.MinLat}}" />
				</CalibrationPoints>
			</MapCalibration>
		</OruxTracker>
		{{end}}
	</MapCalibration>
</OruxTracker>
`
