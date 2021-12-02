package main

import (
	"fmt"

	"github.com/wellmoon/m4aTag/mtag"
)

func main() {
	TestUpdateM4aTag()
}

func TestUpdateM4aTag() {
	mtag.UpdateM4aTag(false, "test/Shape of You.m4a", "Shape of You", "Ed Sheeran", "Shape Of You", "https://t.me/VmomoVBot", "test/vmomov.jpg")
}
func TestReadM4aTag() {
	tagInfo, err := mtag.ReadM4aTag("test/Shape of You.m4a")
	if err != nil {
		fmt.Println("err : + " + err.Error())
	} else {
		fmt.Println(tagInfo.Name)
		fmt.Println(tagInfo.Album)
		fmt.Println(tagInfo.Artist)
	}
}
