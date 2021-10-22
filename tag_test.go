package mtag

import (
	"testing"

	"github.com/wellmoon/m4aTag/mtag"
)

func TestM4aTag(t *testing.T) {
	mtag.UpdateM4aTag(false, "test/Shape of You.m4a", "Shape of You", "Ed Sheeran", "Shape Of You", "https://t.me/VmomoVBot", "test/vmomov.jpg")
}
