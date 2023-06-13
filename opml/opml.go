package opml

import (
	"encoding/xml"
	"time"
)

type Outline struct {
	Title   string `xml:"title"`
	Text    string `xml:"text,attr"`
	XMLURL  string `xml:"xmlUrl,attr"`
	HTMLURL string `xml:"htmlUrl,attr"`
}

type OPML struct {
	Title     string    `xml:"head>title"`
	Outlines  []Outline `xml:"body>outline"`
	CreatedAt time.Time `xml:"dateCreated"`
}

func ParseOPML(data []byte) (opml *OPML, err error) {
	err = xml.Unmarshal(data, &opml)
	if err != nil {
		return
	}
	return
}
