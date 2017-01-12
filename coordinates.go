package maps

import (
	"fmt"
	"math"
	"strconv"
)

// Deg defines a point as degrees (EPSG:4326).
type Deg struct {
	Lat float64 // Latitude (lines connecting the poles): [-85.0511, 85.0511]
	Lon float64 // Longitude (lines around the equator and parallel to it): [-180, 180]
}

const MaxLat float64 = 85.0511

// XY converts d to XY.
func (d Deg) XY(z int) XY {
	x := (d.Lon + 180) / 360 * (math.Exp2(float64(z)))
	y := (1 - math.Log(math.Tan(d.Lat*math.Pi/180)+1/math.Cos(d.Lat*math.Pi/180))/math.Pi) / 2 * (math.Exp2(float64(z)))
	return XY{
		X:  int(x),
		Y:  int(y),
		Z:  z,
		XP: int(256 * (x - float64(int(x)))),
		YP: int(256 * (y - float64(int(y)))),
	}
}

func (d Deg) String() string {
	return fmt.Sprintf("%v°,%v°", d.Lat, d.Lon)
}

// Earth Radius (mean radius defined by IUGG).
// This is not the radius at the equator.
const EarthRadius Meter = 6371008.8

type Meter float64

// String converts the length to a string with suffix m.
func (m Meter) String() string {
	return strconv.FormatFloat(float64(m), 'f', -1, 64) + "m"
}

// Kilometer scales the given length to km and returns a string with the suffix km.
func (m Meter) Kilometer() string {
	return strconv.FormatFloat(float64(m)/1000, 'f', -1, 64) + "km"
}

// Distance calculates the great circle distance between points at d1 and d2 in meters.
// The great circle distance, is the minimal path length between these points on a sphere
// with the nominal earth radius, for a path which is contraint to the sphere's surface.
// The calculation is done using the Vincenty formula.
func (d1 Deg) Distance(d2 Deg) Meter {
	radians := func(deg float64) float64 { return deg / 180 * math.Pi }
	lat1, lon1 := radians(d1.Lat), radians(d1.Lon)
	lat2, lon2 := radians(d2.Lat), radians(d2.Lon)
	dLon := math.Abs(lon2 - lon1)

	// Vincenty formula.
	a := math.Cos(lat2) * math.Sin(dLon)
	b := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	return EarthRadius * Meter(math.Atan2(math.Sqrt(a*a+b*b), math.Sin(lat1)*math.Sin(lat2)+math.Cos(lat1)*math.Cos(lat2)*math.Cos(dLon)))
}

// XY defines the tile coordinates with a zoom index.
type XY struct {
	X, Y   int // Tile index.
	XP, YP int // Pixel offset to top left corner.
	Z      int // Zoom index.
}

// PixelSize calculates the edge length of a single pixel at XY in meters.
// It uses the mean earth Radius instead of the equator length.
func (xy XY) PixelSize() Meter {
	deg := xy.Deg()
	coslat := math.Cos(deg.Lat / 180 * math.Pi)
	// 2 * pi * R    pi * R     |
	// ---------- =  -------    | reduced by factor cos(lat)
	// 256 * 2^z     2^(7+z)    |
	return EarthRadius * Meter(math.Pi*coslat/float64(uint(1<<uint(7+xy.Z))))
}

func (xy XY) String() string {
	return fmt.Sprintf("%d/%d/%d.png:%d,%d", xy.Z, xy.X, xy.Y, xy.XP, xy.YP)
}

// Deg converts xy to Deg for the given zoom level.
func (xy XY) Deg() Deg {
	x := float64(xy.X) + float64(xy.XP)/256
	y := float64(xy.Y) + float64(xy.YP)/256
	n := math.Pi - 2*math.Pi*y/math.Exp2(float64(xy.Z))
	return Deg{
		Lat: 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n))),
		Lon: x/math.Exp2(float64(xy.Z))*360 - 180,
	}
}
