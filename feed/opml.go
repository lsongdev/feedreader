package feed

import (
	"encoding/xml"
	"time"
)

type Outline struct {
	Type    string `xml:"type,attr"`
	Title   string `xml:"title,attr"`
	Text    string `xml:"text,attr"`
	XMLURL  string `xml:"xmlUrl,attr"`
	HTMLURL string `xml:"htmlUrl,attr"`
}

type OPML struct {
	XMLName   xml.Name  `xml:"opml"`
	Title     string    `xml:"head>title"`
	Outlines  []Outline `xml:"body>outline"`
	CreatedAt time.Time `xml:"dateCreated"`
}

func ParseOPML(data []byte) (opml *OPML, err error) {
	err = xml.Unmarshal(data, &opml)
	return
}
