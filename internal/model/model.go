package model

import "encoding/json"

type Source struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	SourceType      string         `json:"sourceType"`
	Folder          string         `json:"folder"`
	DefaultOpenMode string         `json:"defaultOpenMode"`
	DefaultQueue    string         `json:"defaultQueue"`
	FeedURL         string         `json:"feedUrl,omitempty"`
	EmailAlias      string         `json:"emailAlias,omitempty"`
	Config          map[string]any `json:"config,omitempty"`
	Enabled         bool           `json:"enabled"`
	LastFetchedAt   *string        `json:"lastFetchedAt"`
	CreatedAt       string         `json:"createdAt"`
	UpdatedAt       string         `json:"updatedAt"`
}

type PaperLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type QuoteReply struct {
	Text     string `json:"text"`
	URL      string `json:"url,omitempty"`
	Traction string `json:"traction,omitempty"`
}

type DiscussionQuote struct {
	Speaker     string       `json:"speaker"`
	Affiliation string       `json:"affiliation,omitempty"`
	Text        string       `json:"text"`
	URL         string       `json:"url,omitempty"`
	Kind        string       `json:"kind,omitempty"`
	Traction    string       `json:"traction,omitempty"`
	Replies     []QuoteReply `json:"replies,omitempty"`
}

type Item struct {
	ID               string            `json:"id"`
	SourceID         string            `json:"sourceId"`
	ExternalID       string            `json:"externalId"`
	CanonicalURL     string            `json:"canonicalUrl"`
	DiscussionURL    string            `json:"discussionUrl,omitempty"`
	Title            string            `json:"title"`
	Author           string            `json:"author"`
	PublishedAt      string            `json:"publishedAt"`
	PublishedDay     string            `json:"publishedDay"`
	OpenMode         string            `json:"openMode"`
	ActionURL        string            `json:"actionUrl"`
	ContentMarkdown  string            `json:"contentMarkdown,omitempty"`
	PlainText        string            `json:"plainText"`
	Excerpt          string            `json:"excerpt,omitempty"`
	TractionNote     string            `json:"tractionNote,omitempty"`
	DiscussionQuotes []DiscussionQuote `json:"discussionQuotes,omitempty"`
	Metadata         json.RawMessage   `json:"metadata"`
	CreatedAt        string            `json:"createdAt"`
	UpdatedAt        string            `json:"updatedAt"`
}

type UserItemState struct {
	InLater      bool    `json:"inLater"`
	SavedAt      *string `json:"savedAt"`
	ReadAt       *string `json:"readAt"`
	ReadProgress float64 `json:"readProgress"`
}

type Highlight struct {
	ID        string `json:"id"`
	ItemID    string `json:"itemId"`
	Quote     string `json:"quote"`
	Prefix    string `json:"prefix"`
	Suffix    string `json:"suffix"`
	Note      string `json:"note"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Preferences struct {
	Theme       string  `json:"theme"`
	FontFamily  string  `json:"fontFamily"`
	FontSize    float64 `json:"fontSize"`
	LineHeight  float64 `json:"lineHeight"`
	ColumnWidth string  `json:"columnWidth"`
	Align       string  `json:"align"`
	UpdatedAt   string  `json:"updatedAt"`
}

type ItemRow struct {
	Item           Item          `json:"item"`
	Source         Source        `json:"source"`
	State          UserItemState `json:"state"`
	HighlightCount int           `json:"highlightCount"`
}

type Feed struct {
	Day      string         `json:"day"`
	PrevDay  *string        `json:"prevDay"`
	NextDay  *string        `json:"nextDay"`
	Earliest *string        `json:"earliest"`
	Latest   *string        `json:"latest"`
	Counts   map[string]int `json:"counts"`
	Items    []ItemRow      `json:"items"`
}

type InitialState struct {
	InLater *bool   `json:"inLater,omitempty"`
	ReadAt  *string `json:"readAt,omitempty"`
}

type IngestItem struct {
	ID               string            `json:"id,omitempty"`
	SourceID         string            `json:"sourceId,omitempty"`
	ExternalID       string            `json:"externalId,omitempty"`
	CanonicalURL     string            `json:"canonicalUrl"`
	DiscussionURL    string            `json:"discussionUrl,omitempty"`
	Title            string            `json:"title"`
	Author           string            `json:"author,omitempty"`
	PublishedAt      string            `json:"publishedAt,omitempty"`
	OpenMode         string            `json:"openMode,omitempty"`
	ActionURL        string            `json:"actionUrl,omitempty"`
	ContentMarkdown  string            `json:"contentMarkdown,omitempty"`
	PlainText        string            `json:"plainText,omitempty"`
	Excerpt          string            `json:"excerpt,omitempty"`
	TractionNote     string            `json:"tractionNote,omitempty"`
	DiscussionQuotes []DiscussionQuote `json:"discussionQuotes,omitempty"`
	Metadata         json.RawMessage   `json:"metadata,omitempty"`
	InitialState     *InitialState     `json:"initialState,omitempty"`
}

type IngestResult struct {
	Item    Item `json:"item"`
	Created bool `json:"created"`
}

func ValidSourceType(v string) bool {
	switch v {
	case "rss", "newsletter", "youtube", "hn", "papers", "podcast", "snapshot", "manual":
		return true
	default:
		return false
	}
}

func ValidOpenMode(v string) bool {
	switch v {
	case "reader", "external", "deeplink":
		return true
	default:
		return false
	}
}

func ValidQueue(v string) bool {
	return v == "feed" || v == "later"
}

func ValidTheme(v string) bool {
	return v == "light" || v == "warm" || v == "dark"
}

func ValidFont(v string) bool {
	return v == "inter" || v == "lora" || v == "newsreader"
}

func ValidWidth(v string) bool {
	return v == "cozy" || v == "standard" || v == "wide"
}

func ValidAlign(v string) bool {
	return v == "left" || v == "center"
}

func DefaultPreferences(now string) Preferences {
	return Preferences{
		Theme:       "warm",
		FontFamily:  "inter",
		FontSize:    18,
		LineHeight:  1.65,
		ColumnWidth: "standard",
		Align:       "left",
		UpdatedAt:   now,
	}
}
