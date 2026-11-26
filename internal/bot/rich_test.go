package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/dehobitto/muzlan/internal/search"
)

var richTestResults = []search.Result{
	{Title: `Schism <Live>`, Creator: "TOOL", URL: "u1", Thumbnail: "https://i.ytimg.com/vi/a/mqdefault.jpg", Duration: 447 * time.Second},
	{Title: "No channel", URL: "u2"},
}

func richTestDownload(result search.Result) string { return "download:" + result.URL }

func richTestNav(page int) string { return "page:tok:" + string(rune('0'+page)) }

const richTestHeader = `<table bordered><tr><td align="center">&nbsp;<br><b>&lt;b&gt;tool&lt;/b&gt; &amp; co...</b><br>&nbsp;</td></tr></table>`

const richTestFirst = `<table bordered><tr><td>01</td><td><b>Schism &lt;Live&gt;</b><br><sub>TOOL · 7:27</sub></td></tr></table>` +
	`<tg-button-row><tg-button type="callback_data" style="primary" data="download:u1">Download</tg-button></tg-button-row>`

const richTestSecond = `<table bordered><tr><td>02</td><td><b>No channel</b></td></tr></table>` +
	`<tg-button-row><tg-button type="callback_data" style="primary" data="download:u2">Download</tg-button></tg-button-row>`

func TestRichResultsWithThumbnails(t *testing.T) {
	got := richResults(newResultsPage(`<b>tool</b> & co`, richTestResults, 1), true, richTestDownload, richTestNav)

	want := richTestHeader + `<img src="tg://photo?id=thumb0"/>` + richTestFirst + "<hr/>" + richTestSecond
	if got.HTML != want {
		t.Fatalf("unexpected html:\n got %s\nwant %s", got.HTML, want)
	}
	if len(got.Media) != 1 || got.Media[0].ID != "thumb0" || got.Media[0].Media.Media != "https://i.ytimg.com/vi/a/mqdefault.jpg" {
		t.Fatalf("unexpected media: %#v", got.Media)
	}
}

func TestRichResultsWithoutThumbnails(t *testing.T) {
	got := richResults(newResultsPage(`<b>tool</b> & co`, richTestResults, 1), false, richTestDownload, richTestNav)

	if want := richTestHeader + richTestFirst + "<p>&nbsp;</p>" + richTestSecond; got.HTML != want {
		t.Fatalf("unexpected html:\n got %s\nwant %s", got.HTML, want)
	}
	if strings.Contains(got.HTML, "<img") || len(got.Media) != 0 {
		t.Fatalf("expected no thumbnails, got %#v", got)
	}
}

func TestRichResultsPageButtons(t *testing.T) {
	results := make([]search.Result, 12) // 3 pages of 5
	for i := range results {
		results[i] = search.Result{Title: "t", URL: "u"}
	}
	back := `<tg-button type="callback_data" data="page:tok:%d">‹ Back</tg-button>`
	next := `<tg-button type="callback_data" data="page:tok:%d">Next ›</tg-button>`

	tests := []struct {
		page               int
		header, first      string
		wantBack, wantNext bool
	}{
		{page: 1, header: "<sub>1 / 3</sub>", first: "<td>01</td>", wantNext: true},
		{page: 2, header: "<sub>2 / 3</sub>", first: "<td>06</td>", wantBack: true, wantNext: true},
		{page: 3, header: "<sub>3 / 3</sub>", first: "<td>11</td>", wantBack: true},
	}
	for _, test := range tests {
		got := richResults(newResultsPage("q", results, test.page), true, richTestDownload, richTestNav).HTML
		assertContains(t, got, test.header, test.first)
		if strings.Contains(got, strings.Replace(back, "%d", string(rune('0'+test.page-1)), 1)) != test.wantBack ||
			strings.Contains(got, strings.Replace(next, "%d", string(rune('0'+test.page+1)), 1)) != test.wantNext {
			t.Fatalf("page %d: unexpected page buttons:\n%s", test.page, got)
		}
	}

	single := richResults(newResultsPage("q", results[:5], 1), true, richTestDownload, richTestNav).HTML
	if strings.Contains(single, "<sub>1 / 1</sub>") || strings.Contains(single, "page:") {
		t.Fatalf("single page must have no page number or buttons:\n%s", single)
	}
}

func TestFormatDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		5 * time.Second:    "0:05",
		447 * time.Second:  "7:27",
		3725 * time.Second: "1:02:05",
	} {
		if got := formatDuration(d); got != want {
			t.Fatalf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func assertContains(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in:\n%s", want, got)
		}
	}
}
