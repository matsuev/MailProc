package main

import (
	"bytes"
	"fmt"
	"log"
	"net/mail"
	"os"

	message "github.com/matsuev/go-message"
	charset "github.com/matsuev/go-message/charset"
)

func main() {

	hh := []string{
		"MIME-Version",
		"Message-Id",
		"Content-Type",
		"Content-Transfer-Encoding",
		"In-Reply-To",
		"References",
		"Subject",
	}

	// Открытие файла на чтение
	r, err := os.Open("./testmail.msg")
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	// Читаем сообщение из файла
	msg, err := message.Read(r)
	if err != nil {
		log.Fatal(err)
	}

	from, err := getMailHeader("From", msg.Header)
	if err != nil {
		log.Fatal(err)
	}

	to, err := getMailHeader("To", msg.Header)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(from)
	fmt.Println(to)

	newHeader := make(message.Header)
	for _, hk := range hh {
		if hv := msg.Header.Get(hk); hv != "" {
			newHeader.Set(hk, hv)
		}
	}

	var b bytes.Buffer
	w, err := message.CreateWriter(&b, newHeader)

	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	fmt.Println(b.String())
}

func getMailHeader(k string, h message.Header) (*mail.Address, error) {
	dh, err := charset.DecodeHeader(h.Get(k))
	if err != nil {
		return nil, err
	}

	rh, err := mail.ParseAddress(dh)
	return rh, err
}
