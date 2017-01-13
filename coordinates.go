package maps

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

// Degree is a unit for angles.
// It represent distinct values in the range [-180, 180).
type Degree float64

// String prints a degree value with the "°" suffix.
func (d Degree) String() string {
	return strconv.FormatFloat(float64(d), 'f', -1, 64) + "°"
}

// Radians converts a Degree value to radians [-pi, pi].
func (d Degree) Radians() float64 {
	return float64(d) / 180 * math.Pi
}

// LatLon defines a point by the spherical angles Lat and Lon given in degree (EPSG:4326).
// Changing the lateral coordinate from -90 to 90 is equivalent from a point moving from the South to the North pole.
// The longitudinal coordinate is 0 at the Greenwich meridian and increases by moving East.
type LatLon struct {
	Lat Degree // Latitude (lines connecting the poles) "[-90, 90]"
	Lon Degree // Longitude (lines around the equator and parallel to it): "[-180, 180]"
}

// XY converts d to XY for the given zoom level [0, 24].
func (d LatLon) XY(z int) (XY, error) {
	if z < 0 || z > 24 {
		return XY{}, ZoomRangeError
	}
	if d.Lat < MinLatitude(z) || d.Lat > MaxLatitude {
		return XY{}, fmt.Errorf("latitude %s value cannot be represented by tile coordinates", d.Lat)
	}
	x := (float64(d.Lon) + 180) / 360 * two[z]
	y := (1 - math.Log(math.Tan(d.Lat.Radians())+1/math.Cos(d.Lat.Radians()))/math.Pi) / 2 * two[z]
	return XY{
		X:  int(x),
		Y:  int(y),
		Z:  z,
		XP: int(256 * (x - float64(int(x)))),
		YP: int(256 * (y - float64(int(y)))),
	}, nil
}

func (d LatLon) String() string {
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
func (d1 LatLon) Distance(d2 LatLon) Meter {
	lat1, lon1 := d1.Lat.Radians(), d1.Lon.Radians()
	lat2, lon2 := d2.Lat.Radians(), d2.Lon.Radians()
	dLon := math.Abs(lon2 - lon1)

	// Vincenty formula.
	a := math.Cos(lat2) * math.Sin(dLon)
	b := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	return EarthRadius * Meter(math.Atan2(math.Sqrt(a*a+b*b), math.Sin(lat1)*math.Sin(lat2)+math.Cos(lat1)*math.Cos(lat2)*math.Cos(dLon)))
}

// XY defines the tile coordinates with a zoom index [0,24].
// The tile coordinate system does not have the same range as the spherical degrees.
// Points north of MaxLatitude or south of MinLatitude(zoomLevel) cannot be represented.
// The longitudinal limits for the representable values are -180° to 179.99999991618097° for Z = 24
// and 178.59375° for Z = 0.
type XY struct {
	X, Y   int // Tile index [0, 2^Z].
	XP, YP int // Pixel offset to top left corner within the tile [0, 255].
	Z      int // Zoom index [0, 24].
}

// MaxLatitude is the maximal latitude, that a tile coordinate can represent
const MaxLatitude Degree = 85.05112877980659

// MinLatitude returns the minimal latitude, that a tile coordinate can represent
// for the given zoom level.
// The value is -85.0511287725758° for full resolution (z=24)
// and -84.92832092949963° for z = 0.
func MinLatitude(z int) Degree {
	checkZoom(z)
	return XY{X: (1 << uint(z)) - 1, Y: (1 << uint(z)) - 1, Z: z, XP: 255, YP: 255}.LatLon().Lat
}

// PixelSize calculates the edge length of a single pixel at XY in meters.
// It uses the mean earth Radius instead of the equator length for the calculation.
func (xy XY) PixelSize() Meter {
	// 2 * pi * R    pi * R     |
	// ---------- =  -------    | reduced by factor cos(lat)
	// 256 * 2^z     2^(7+z)    |
	deg := xy.LatLon()
	coslat := math.Cos(deg.Lat.Radians())
	// This does not overflow for max zoom = 24.
	checkZoom(xy.Z)
	return EarthRadius * Meter(math.Pi*coslat/float64(uint(1)<<uint(7+xy.Z)))
}

func (xy XY) String() string {
	return fmt.Sprintf("%d/%d/%d.png:%d,%d", xy.Z, xy.X, xy.Y, xy.XP, xy.YP)
}

// Deg converts xy to LatLon for the given zoom level.
func (xy XY) LatLon() LatLon {
	x := float64(xy.X) + float64(xy.XP)/256
	y := float64(xy.Y) + float64(xy.YP)/256
	n := math.Pi - 2*math.Pi*y/two[xy.Z]
	return LatLon{
		Lat: Degree(180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))),
		Lon: Degree(x/two[xy.Z]*360 - 180),
	}
}

var ZoomRangeError = errors.New("zoom value is out of range [0, 24]")

// checkZoom panics, if the zoom value is out of range.
func checkZoom(z int) {
	if z < 0 || z > 24 {
		panic("zoom value is out of range [0, 24]")
	}
}

// Two stores the numbers 2^z for the zoom levels 0..24.
var two [25]float64

func init() {
	for i := 0; i < len(two); i++ {
		two[i] = float64(uint(1) << uint(i))
	}
}
