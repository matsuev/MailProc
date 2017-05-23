package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/mail"
	"os"
	"strconv"
	"strings"

	message "github.com/emersion/go-message"
	"github.com/jhillyerd/go.enmime"
)

func main() {
	r, err := os.Open("./testmsg.eml")
	if err != nil {
		log.Fatal(err)
	}

	// Let's assume r is an io.Reader that contains a message.
	// var r io.Reader

	m, err := message.Read(r)
	if err != nil {
		log.Fatal(err)
	}

	hh := []string{
		"MIME-Version",
		"Message-Id",
		"Content-Type",
		"Content-Transfer-Encoding",
		"In-Reply-To",
		"References",
		// "Subject",
	}

	var newHeader string
	// var newHeader message.Header
	// newHeader = make(message.Header)

	for _, k := range hh {
		if h := m.Header.Get(k); h != "" {
			newHeader += k + ": " + h + "\r\n"
			// newHeader.Set(k, h)
		}
	}
	newHeader += "\r\n"

	mm, err := message.Read(bytes.NewReader([]byte(newHeader)))
	if err != nil {
		log.Fatal(err)
	}

	from, _ := mail.ParseAddress(enmime.DecodeHeader(m.Header.Get("From")))
	to, _ := mail.ParseAddress(enmime.DecodeHeader(m.Header.Get("To")))
	subj := enmime.DecodeHeader(m.Header.Get("Subject"))
	// subj := mime.BEncoding.Encode("utf-8", enmime.DecodeHeader(m.Header.Get("Subject")))

	var lprefix string = "TEST: Тестовая рассылка"
	var uid uint64 = 145

	from.Name = fmt.Sprintf("%s", lprefix)
	from.Address = to.Address

	mm.Header.Add("From", fmt.Sprintf("%s <%s>", from.Name, from.Address))
	mm.Header.Add("Reply-To", to.Address)
	mm.Header.Add("Subject", subj)
	mm.Header.Add("X-KLSH-Sender", strconv.FormatUint(uid, 10))

	// newHeader += fmt.Sprintf("From: %s\r\n", from.String())
	// newHeader += fmt.Sprintf("Reply-To: <%s>\r\n", to.Address)
	// newHeader += fmt.Sprintf("Subject: %s\r\n", subj)
	// newHeader += fmt.Sprintf("X-KLSH-Sender: %v\r\n", uid)

	// newHeader += fmt.Sprintf("From: %s\r\n", from.String())
	// newHeader += fmt.Sprintf("Reply-To: <%s>\r\n", to.Address)
	// newHeader += fmt.Sprintf("Subject: %s\r\n", subj)
	// newHeader += fmt.Sprintf("X-KLSH-Sender: %v\r\n", uid)
	// newHeader += "\r\n"
	// // fmt.Printf("Subj: %s\n", subj)

	// fmt.Println(newHeader)

	// We'll add "This message is powered by Go" at the end of each text entity.
	poweredByHtml := `<p><b>Сообщение от:</b> Александр Мацуев
	  &lt;<a href="mailto:alex.matsuev@gmail.com">alex.matsuev@gmail.com</a>&gt;<p>`

	poweredByPlain := "Сообщение от:  Александр Мацуев <alex.matsuev@gmail.com>\n— — — — — —\n\n"

	// hdr := make(message.Header)
	var b bytes.Buffer
	w, err := message.CreateWriter(&b, mm.Header)
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	// Define a function that transforms message.
	var transform func(w *message.Writer, e *message.Entity) error

	transform = func(w *message.Writer, e *message.Entity) error {
		if mr := e.MultipartReader(); mr != nil {
			// This is a multipart entity, transform each of its parts
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				} else if err != nil {
					return err
				}

				pw, err := w.CreatePart(p.Header)
				if err != nil {
					return err
				}

				if err := transform(pw, p); err != nil {
					return err
				}

				pw.Close()
			}
			return nil
		} else {
			body := e.Body
			if strings.HasPrefix(e.Header.Get("Content-Type"), "text/plain") {
				body = io.MultiReader(strings.NewReader(poweredByPlain), body)
			}
			if strings.HasPrefix(e.Header.Get("Content-Type"), "text/html") {
				body = io.MultiReader(strings.NewReader(poweredByHtml), body)
			}
			_, err := io.Copy(w, body)
			return err
		}
	}

	if err := transform(w, m); err != nil {
		log.Fatal(err)
	}

	fmt.Println(b.String())
}
