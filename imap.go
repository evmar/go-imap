package main

import (
	"bufio"
	"crypto/tls"
	"os"
	"log"
	"strings"
	"fmt"
	"net/textproto"
	"io"
	"strconv"
)

func check(err os.Error) {
	if err != nil {
		panic(err)
	}
}

type Status int
const (
	OK = Status(iota)
	NO
	BAD
)
func (s Status) String() string {
	return []string{
		"OK",
		"NO",
		"BAD",
	}[s];
}

type Tag int
const Untagged = Tag(-1)

type IMAP struct {
	r *textproto.Reader
	w io.Writer
}

func splitToken(text string) (string, string) {
	space := strings.Index(text, " ")
	if space < 0 {
		return text, ""
	}
	return text[:space], text[space+1:]
}

func (imap *IMAP) ReadLine() (Tag, string, os.Error) {
	line, err := imap.r.ReadLine()
	if err != nil {
		return Untagged, "", err
	}

	switch line[0] {
	case '*':
		return Untagged, line[2:], nil
	case 'a':
		tagstr, text := splitToken(line)
		tagnum, err := strconv.Atoi(tagstr)
		if err != nil {
			return Untagged, "", err
		}
		return Tag(tagnum), text, nil
	}

	return Untagged, "", fmt.Errorf("unexpected response %q", line)
}

func (imap *IMAP) ReadStatus() (Status, string, os.Error) {
	tag, text, err := imap.ReadLine()
	if err != nil {
		return BAD, "", err
	}
	if tag != Untagged {
		return BAD, "", fmt.Errorf("got tagged response %q when expecting status", text)
	}

	codes := map[string]Status{
		"OK": OK,
		"NO": NO,
		"BAD": BAD,
	}
	code, text := splitToken(text)

	status, known := codes[code]
	if !known {
		return BAD, "", fmt.Errorf("unexpected status %q", code)
	}
	return status, text, nil
}

/*
func (p *IMAP) Command(cmd string) os.Error {
	w.Write(
	return nil
}

func (p *IMAP) Auth(user string, pass string) os.Error {
	return nil
}
*/

func main() {
	log.Printf("dial")
	conn, err := tls.Dial("tcp", "imap.gmail.com:993", nil)
	check(err)

	r := textproto.NewReader(bufio.NewReader(conn))
	p := IMAP{r, conn}

	status, text, err := p.ReadStatus()
	check(err)
	log.Printf("%v %v %v", status, text, err)
}
