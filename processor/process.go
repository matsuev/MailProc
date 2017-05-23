package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"

	message "github.com/emersion/go-message"
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

	// We'll add "This message is powered by Go" at the end of each text entity.
	poweredByHtml := `<p><b>Сообщение от:</b> Александр Мацуев
    &lt;<a href="mailto:alex.matsuev@gmail.com">alex.matsuev@gmail.com</a>&gt;<p>`

	poweredByPlain := "Сообщение от:  Александр Мацуев <alex.matsuev@gmail.com>\n— — — — — —\n\n"

	var b bytes.Buffer
	w, err := message.CreateWriter(&b, m.Header)
	if err != nil {
		log.Fatal(err)
	}

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
	w.Close()

	log.Println(b.String())
}
