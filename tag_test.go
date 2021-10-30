package mtag

import (
	"fmt"
	"testing"

	"github.com/wellmoon/m4aTag/mtag"
)

func TestUpdateM4aTag(t *testing.T) {
	mtag.UpdateM4aTag(false, "test/Shape of You.m4a", "Shape of You", "Ed Sheeran", "Shape Of You", "https://t.me/VmomoVBot", "test/vmomov.jpg")
}
func TestReadM4aTag(t *testing.T) {
	tagInfo, err := mtag.Read("test/Shape of You.m4a")
	if err != nil {
		fmt.Println("err : + " + err.Error())
	} else {
		fmt.Println(tagInfo.Name)
		fmt.Println(tagInfo.Album)
		fmt.Println(tagInfo.Artist)
	}
}
