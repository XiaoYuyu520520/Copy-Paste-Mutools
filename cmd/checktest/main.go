package main

import (
	"fmt"
	"image/png"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/software"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := test.NewApp()
	a.Settings().SetTheme(theme.LightTheme())

	c1 := widget.NewCheck("提示词", nil)
	c1.SetChecked(true)
	c2 := widget.NewCheck("语句", nil)
	c2.SetChecked(true)
	l := widget.NewLabel("第 1 行")

	vbox := container.NewVBox(c1, c2, l)
	border := container.NewBorder(nil, nil, vbox, nil, widget.NewLabel("右侧"))

	c := software.NewCanvas()
	c.SetContent(border)
	c.Resize(fyne.NewSize(400, 200))
	time.Sleep(150 * time.Millisecond)

	img := c.Capture()
	f, err := os.Create("screenshots/test_check.png")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("wrote screenshots/test_check.png")
}
