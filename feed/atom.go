package feed

import (
	"encoding/xml"
	"strings"
)

// AtomFeed represents an atom web feed.
type AtomFeed struct {
	// XMLName.
	XMLName xml.Name `xml:"feed"`

	// Universally unique feed ID (required).
	ID string `xml:"id"`

	// Human readable title for the feed (required).
	Title AtomText `xml:"title"`

	// Last time the feed was significantly modified (required).
	Updated string `xml:"updated"`

	// Entries for the feed (required).
	Entries []AtomEntry `xml:"entry"`

	// Authors of the feed (recommended).
	Authors []AtomPerson `xml:"author,omitempty"`

	// Links which identify related web pages (recommended).
	Links []AtomLink `xml:"link"`

	// Categories the feed belongs to (optional).
	Categories []AtomCategory `xml:"category,omitempty"`

	// Contributors to the feed (optional).
	Contributors []AtomPerson `xml:"contributor,omitempty"`

	// Software used to generate the feed (optional).
	Generator AtomGenerator `xml:"generator,omitempty"`

	// Small icon used for visual identification (optional).
	Icon string `xml:"icon,omitempty"`

	// Larger logo for visual identification (optional).
	Logo string `xml:"logo,omitempty"`

	// Information about rights, for example copyrights (optional).
	Rights AtomText `xml:"rights,omitempty"`

	// Human readable description or subtitle (optional).
	Subtitle AtomText `xml:"subtitle,omitempty"`
}

// AtomEntry represents an atom entry.
type AtomEntry struct {
	// Universally unique feed ID (required).
	ID string `xml:"id"`

	// Human readable title for the entry (required).
	Title AtomText `xml:"title"`

	// Last time the feed was significantly modified (required).
	Updated string `xml:"updated"`

	// Authors of the entry (recommended).
	Authors []AtomPerson `xml:"author,omitempty"`

	// Content of the entry (recommended).
	Content AtomText `xml:"content"`

	// Links which identify related web pages (recommended).
	Links []AtomLink `xml:"link"`

	// Short summary, abstract or excerpt of the entry (recommended).
	Summary AtomText `xml:"summary,omitempty"`

	// Categories the entry belongs too (optional).
	Categories []AtomCategory `xml:"category,omitempty"`

	// Contributors to the entry (optional).
	Contributors []AtomPerson `xml:"contributor,omitempty"`

	// Time of the initial creation of the entry (optional).
	Published string `xml:"published,omitempty"`

	// Information about rights, for example copyrights (optional).
	Rights AtomText `xml:"rights,omitempty"`
}

func (entry *AtomEntry) GetContent() (content string) {
	content = strings.TrimSpace(entry.Content.Data)
	if content != "" {
		return
	}
	content = strings.TrimSpace(entry.Content.InnerXML)
	if content != "" {
		return
	}
	content = strings.TrimSpace(entry.Summary.Data)
	if content != "" {
		return
	}
	content = strings.TrimSpace(entry.Summary.InnerXML)
	if content != "" {
		return
	}
	return
}

// AtomLink represents the atom link tag.
type AtomLink struct {
	// Hypertext reference (required).
	Href string `xml:"href,attr"`

	// Single Link relation type (optional).
	Rel string `xml:"rel,attr,omitempty"`

	// Media type of the resource (optional).
	Type string `xml:"type,attr,omitempty"`

	// Language of referenced resource (optional).
	HrefLang string `xml:"hreflang,attr,omitempty"`

	// Human readable information about the link (optional).
	Title string `xml:"title,attr,omitempty"`

	// Length of the resource in bytes (optional).
	Length string `xml:"length,attr,omitempty"`
}

// AtomPerson represents a person, corporation, et cetera.
type AtomPerson struct {
	// Human readable name for the person (required).
	Name string `xml:"name"`

	// Home page for the person (optional).
	URI string `xml:"uri,omitempty"`

	// Email address for the person (optional).
	Email string `xml:"email,omitempty"`
}

// AtomCategory identifies the category.
type AtomCategory struct {
	// Identifier for this category (required).
	Term string `xml:"term,attr"`

	// Categorization scheme via a URI (optional).
	Scheme string `xml:"scheme,attr,omitempty"`

	// Human readable label for display (optional).
	Label string `xml:"label,attr,omitempty"`
}

// AtomGenerator identifies the generator.
type AtomGenerator struct {
	// Generator name (required).
	Name string `xml:",chardata"`

	// URI for this generator (optional).
	URI string `xml:"uri,attr,omitempty"`

	// Version for this generator (optional).
	Version string `xml:"version,attr,omitempty"`
}

// AtomText identifies human readable text.
type AtomText struct {
	// Text body (required).
	Data string `xml:",chardata"`

	// InnerXML data (optional).
	InnerXML string `xml:",innerxml"`

	// Text type (optional).
	Type string `xml:"type,attr,omitempty"`

	// URI where the content can be found (optional for <content>).
	URI string `xml:"uri,attr,omitempty"`
}

// parseAtom parses an atom feed and returns a generic feed.
func ParseAtom(data []byte) (feed *AtomFeed, err error) {
	err = xml.Unmarshal(data, &feed)
	return
}
