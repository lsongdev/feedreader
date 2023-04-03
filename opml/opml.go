package opml

import "encoding/xml"

type Outline struct {
	Text   string `xml:"text,attr"`
	XMLURL string `xml:"xmlUrl,attr"`
}

type OPML struct {
	Outlines []Outline `xml:"body>outline"`
}

func ParseOPML(data []byte) (opml *OPML, err error) {
	err = xml.Unmarshal(data, &opml)
	if err != nil {
		return
	}
	return
}
