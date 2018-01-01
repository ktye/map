package orux

import (
	"os"
	"testing"

	"github.com/ktye/maps"
)

func TestOrux(t *testing.T) {

	var ts maps.LocalTileServer = "test"
	m := Map{
		TopLeft:     maps.LatLon{53.58914, 9.99786},
		BottomRight: maps.LatLon{53.57668, 10.01678},
		ZoomLevels:  []int{13, 15},
	}

	if err := m.Encode("Alster", ts); err != nil {
		t.Fatal(err)
	}

	os.RemoveAll("Alster")
}
