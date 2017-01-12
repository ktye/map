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

// XY converts d to XY for the given zoom level [0, 24].
func (d Deg) XY(z int) XY {
	checkZoom(z)
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

// XY defines the tile coordinates with a zoom index [0,24].
type XY struct {
	X, Y   int // Tile index.
	XP, YP int // Pixel offset to top left corner.
	Z      int // Zoom index.
}

// PixelSize calculates the edge length of a single pixel at XY in meters.
// It uses the mean earth Radius instead of the equator length for the calculation.
// 	Z   Hor. Pixels  PixelSize
// 	0   256          156368.08157840566m
// 	1   512          78184.04078920283m
// 	2   1024         39092.020394601415m
// 	3   2048         19546.010197300708m
// 	4   4096         9773.005098650354m
// 	5   8192         4886.502549325177m
// 	6   16384        2443.2512746625885m
// 	7   32768        1221.6256373312942m
// 	8   65536        610.8128186656471m
// 	9   131072       305.40640933282356m
// 	10  262144       152.70320466641178m
// 	11  524288       76.35160233320589m
// 	12  1048576      38.175801166602945m
// 	13  2097152      19.087900583301472m
// 	14  4194304      9.543950291650736m
// 	15  8388608      4.771975145825368m
// 	16  16777216     2.385987572912684m
// 	17  33554432     1.192993786456342m
// 	18  67108864     0.596496893228171m
// 	19  134217728    0.2982484466140855m
// 	20  268435456    0.14912422330704275m
// 	21  536870912    0.07456211165352138m
// 	22  1073741824   0.03728105582676069m
// 	23  2147483648   0.018640527913380344m
// 	24  4294967296   0.009320263956690172m
func (xy XY) PixelSize() Meter {
	// 2 * pi * R    pi * R     |
	// ---------- =  -------    | reduced by factor cos(lat)
	// 256 * 2^z     2^(7+z)    |
	deg := xy.Deg()
	coslat := math.Cos(deg.Lat / 180 * math.Pi)
	// This does not overflow for max zoom = 24.
	checkZoom(xy.Z)
	return EarthRadius * Meter(math.Pi*coslat/float64(uint(1<<uint(7+xy.Z))))
}

func (xy XY) String() string {
	return fmt.Sprintf("%d/%d/%d.png:%d,%d", xy.Z, xy.X, xy.Y, xy.XP, xy.YP)
}

// Deg converts xy to Deg for the given zoom level.
func (xy XY) Deg() Deg {
	checkZoom(xy.Z)
	x := float64(xy.X) + float64(xy.XP)/256
	y := float64(xy.Y) + float64(xy.YP)/256
	n := math.Pi - 2*math.Pi*y/math.Exp2(float64(xy.Z))
	return Deg{
		Lat: 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n))),
		Lon: x/math.Exp2(float64(xy.Z))*360 - 180,
	}
}

// checkZoom panics, if the zoom value is out of range.
func checkZoom(z int) {
	if z < 0 || z > 24 {
		panic("zoom value is out of range [0, 24]")
	}
}
