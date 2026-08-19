package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/cli-printing-press/v4/internal/naming"
	"github.com/stretchr/testify/require"
)

// TestGeneratedClientDecodesXMLResponsesToJSON pins the fix for CLIs printed
// against APIs that answer in XML rather than JSON. Before it, the raw
// "<feed ...>" bytes were handed back inside a json.RawMessage, reached
// assertLiveJSONBody, and surfaced as "not authenticated or session expired;
// API returned HTML instead of JSON" — a false auth failure on a public,
// unauthenticated endpoint. Reproduced live against arXiv
// (https://export.arxiv.org/api/query, Content-Type: application/atom+xml),
// which needs no credentials at all.
func TestGeneratedClientDecodesXMLResponsesToJSON(t *testing.T) {
	t.Parallel()

	apiSpec := minimalSpec("xmlresponse")
	outputDir := filepath.Join(t.TempDir(), naming.CLI(apiSpec.Name))
	require.NoError(t, New(apiSpec, outputDir).Generate())

	const behaviorTest = `package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xmlresponse-pp-cli/internal/config"
)

// atomFeed mirrors the shape arXiv returns: an Atom feed carrying the
// OpenSearch pagination namespace and repeated <entry> elements.
const atomFeed = ` + "`" + `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">
  <title>arXiv Query: search_query=all:electron</title>
  <opensearch:totalResults>185098</opensearch:totalResults>
  <opensearch:startIndex>0</opensearch:startIndex>
  <opensearch:itemsPerPage>2</opensearch:itemsPerPage>
  <entry>
    <id>http://arxiv.org/abs/1706.03762v7</id>
    <title>Attention Is All You Need</title>
    <published>2017-06-12T17:57:34Z</published>
    <summary>The dominant sequence transduction models</summary>
    <author><name>Ashish Vaswani</name></author>
    <author><name>Noam Shazeer</name></author>
    <link href="https://arxiv.org/abs/1706.03762v7" rel="alternate" type="text/html"/>
    <link href="https://arxiv.org/pdf/1706.03762v7" rel="related" title="pdf"/>
    <category term="cs.CL"/>
    <category term="cs.LG"/>
  </entry>
  <entry>
    <id>http://arxiv.org/abs/cond-mat/0011267v1</id>
    <title>The electronic structure of cuprates</title>
    <published>2000-11-15T16:19:15Z</published>
    <summary>We report studies of the electronic structure</summary>
    <author><name>Mark S. Golden</name></author>
    <link href="https://arxiv.org/abs/cond-mat/0011267v1" rel="alternate" type="text/html"/>
    <category term="cond-mat.supr-con"/>
  </entry>
</feed>` + "`" + `

func getBody(t *testing.T, contentType, body string) json.RawMessage {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	c.cacheDir = t.TempDir()

	data, err := c.Get(context.Background(), "/query", nil)
	if err != nil {
		t.Fatalf("Get(%s) returned error: %v", contentType, err)
	}
	return data
}

func TestAtomXMLResponseDecodesToJSON(t *testing.T) {
	data := getBody(t, "application/atom+xml; charset=utf-8", atomFeed)

	if !json.Valid(data) {
		t.Fatalf("atom+xml response did not become valid JSON, got: %s", data)
	}

	var out struct {
		Feed struct {
			Title        string ` + "`" + `json:"title"` + "`" + `
			TotalResults string ` + "`" + `json:"totalResults"` + "`" + `
			StartIndex   string ` + "`" + `json:"startIndex"` + "`" + `
			ItemsPerPage string ` + "`" + `json:"itemsPerPage"` + "`" + `
			Entry        []struct {
				ID        string ` + "`" + `json:"id"` + "`" + `
				Title     string ` + "`" + `json:"title"` + "`" + `
				Published string ` + "`" + `json:"published"` + "`" + `
				Summary   string ` + "`" + `json:"summary"` + "`" + `
				Author    []struct {
					Name string ` + "`" + `json:"name"` + "`" + `
				} ` + "`" + `json:"author"` + "`" + `
				Category []struct {
					Term string ` + "`" + `json:"@term"` + "`" + `
				} ` + "`" + `json:"category"` + "`" + `
				Link []struct {
					Href string ` + "`" + `json:"@href"` + "`" + `
					Rel  string ` + "`" + `json:"@rel"` + "`" + `
				} ` + "`" + `json:"link"` + "`" + `
			} ` + "`" + `json:"entry"` + "`" + `
		} ` + "`" + `json:"feed"` + "`" + `
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decoding converted feed: %v\ngot: %s", err, data)
	}

	feed := out.Feed
	if feed.Title != "arXiv Query: search_query=all:electron" {
		t.Fatalf("feed title = %q, want the Atom <title> text", feed.Title)
	}
	// The OpenSearch pagination namespace must survive as plain local names.
	if feed.TotalResults != "185098" || feed.StartIndex != "0" || feed.ItemsPerPage != "2" {
		t.Fatalf("opensearch pagination lost: totalResults=%q startIndex=%q itemsPerPage=%q",
			feed.TotalResults, feed.StartIndex, feed.ItemsPerPage)
	}
	// Repeated sibling elements must collapse into an array, not overwrite.
	if len(feed.Entry) != 2 {
		t.Fatalf("feed has %d entries, want 2 (repeated <entry> must become a JSON array)", len(feed.Entry))
	}
	first := feed.Entry[0]
	if first.ID != "http://arxiv.org/abs/1706.03762v7" {
		t.Fatalf("first entry id = %q", first.ID)
	}
	if first.Title != "Attention Is All You Need" {
		t.Fatalf("first entry title = %q", first.Title)
	}
	if first.Published != "2017-06-12T17:57:34Z" {
		t.Fatalf("first entry published = %q", first.Published)
	}
	if first.Summary != "The dominant sequence transduction models" {
		t.Fatalf("first entry summary = %q", first.Summary)
	}
	if len(first.Author) != 2 || first.Author[0].Name != "Ashish Vaswani" {
		t.Fatalf("first entry authors = %+v, want both <author><name> values", first.Author)
	}
	// Attributes are addressable as "@name" keys.
	if len(first.Category) != 2 || first.Category[0].Term != "cs.CL" || first.Category[1].Term != "cs.LG" {
		t.Fatalf("first entry categories = %+v, want both terms as @term", first.Category)
	}
	if len(first.Link) != 2 || first.Link[0].Href != "https://arxiv.org/abs/1706.03762v7" || first.Link[0].Rel != "alternate" {
		t.Fatalf("first entry links = %+v, want href/rel attributes as @href/@rel", first.Link)
	}

	// Cardinality must not wobble inside one response. The second entry has a
	// single <author>, <category>, and <link> where the first has two of each;
	// all must decode to arrays, or a consumer indexing .entry[].author[0]
	// breaks on the second row. This is the shape arXiv really returns.
	second := feed.Entry[1]
	if len(second.Author) != 1 || second.Author[0].Name != "Mark S. Golden" {
		t.Fatalf("second entry authors = %+v, want a one-element array, not a bare object", second.Author)
	}
	if len(second.Category) != 1 || second.Category[0].Term != "cond-mat.supr-con" {
		t.Fatalf("second entry categories = %+v, want a one-element array, not a bare object", second.Category)
	}
	if len(second.Link) != 1 || second.Link[0].Rel != "alternate" {
		t.Fatalf("second entry links = %+v, want a one-element array, not a bare object", second.Link)
	}
	// Nothing repeats <title>, so it must stay a plain string.
	if second.Title != "The electronic structure of cuprates" {
		t.Fatalf("second entry title = %q", second.Title)
	}
}

func TestPlainAndVendorXMLContentTypesDecode(t *testing.T) {
	for _, ct := range []string{"application/xml", "text/xml", "application/rss+xml"} {
		data := getBody(t, ct, ` + "`" + `<rss><channel><item>one</item><item>two</item></channel></rss>` + "`" + `)
		if !json.Valid(data) {
			t.Fatalf("%s response did not become valid JSON, got: %s", ct, data)
		}
		var out struct {
			RSS struct {
				Channel struct {
					Item []string ` + "`" + `json:"item"` + "`" + `
				} ` + "`" + `json:"channel"` + "`" + `
			} ` + "`" + `json:"rss"` + "`" + `
		}
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("decoding %s: %v\ngot: %s", ct, err, data)
		}
		if got := out.RSS.Channel.Item; len(got) != 2 || got[0] != "one" || got[1] != "two" {
			t.Fatalf("%s items = %v, want [one two]", ct, got)
		}
	}
}

func TestHTMLResponsesAreNotXMLDecoded(t *testing.T) {
	// text/html belongs to the response_format:html extraction path, which
	// reads markup. Converting it here would break those CLIs.
	const page = ` + "`" + `<html><body><p>hello</p></body></html>` + "`" + `
	for _, ct := range []string{"text/html; charset=utf-8", "application/xhtml+xml"} {
		if got := string(getBody(t, ct, page)); got != page {
			t.Fatalf("%s body was rewritten to %q, want the markup untouched", ct, got)
		}
	}
}

func TestNonXMLBodyWithXMLContentTypePassesThrough(t *testing.T) {
	// A Content-Type that lies about the payload must not become a hard error.
	const body = ` + "`" + `{"ok":true}` + "`" + `
	if got := string(getBody(t, "application/xml", body)); got != body {
		t.Fatalf("mislabelled JSON body was rewritten to %q, want %q", got, body)
	}
}

func TestDeeplyNestedXMLDoesNotExhaustTheStack(t *testing.T) {
	// A response body is untrusted input. Pathological nesting must degrade to
	// an untouched passthrough, not a stack overflow.
	var b strings.Builder
	const depth = 50000
	for i := 0; i < depth; i++ {
		b.WriteString("<a>")
	}
	for i := 0; i < depth; i++ {
		b.WriteString("</a>")
	}
	body := b.String()
	if got := string(getBody(t, "application/xml", body)); got != body {
		t.Fatalf("deeply nested XML was rewritten (len %d), want the body untouched", len(got))
	}
}

func TestJSONResponsesAreUntouched(t *testing.T) {
	const body = ` + "`" + `{"items":[{"id":1}]}` + "`" + `
	if got := string(getBody(t, "application/json", body)); got != body {
		t.Fatalf("JSON body was rewritten to %q, want %q", got, body)
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "internal", "client", "xml_response_test.go"), []byte(behaviorTest), 0o644))

	runGoCommand(t, outputDir, "test", "./internal/client")
}
