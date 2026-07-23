package main

import (
	"fmt"
	"os"

	qrcode "github.com/skip2/go-qrcode"
)

func showQR(url string) {
	q, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating QR code: %v\n", err)
		return
	}

	fmt.Println(q.ToSmallString(false))
}
