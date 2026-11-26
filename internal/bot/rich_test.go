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

func richTestCallback(result search.Result) string { return "download:" + result.URL }

const richTestHeader = `<table bordered><tr><td align="center">&nbsp;<br><b>&lt;b&gt;tool&lt;/b&gt; &amp; co...</b><br>&nbsp;</td></tr></table>`

const richTestFirst = `<table bordered><tr><td>01</td><td><b>Schism &lt;Live&gt;</b><br><sub>TOOL · 7:27</sub></td></tr></table>` +
	`<tg-button-row><tg-button type="callback_data" style="primary" data="download:u1">Download</tg-button></tg-button-row>`

const richTestSecond = `<table bordered><tr><td>02</td><td><b>No channel</b></td></tr></table>` +
	`<tg-button-row><tg-button type="callback_data" style="primary" data="download:u2">Download</tg-button></tg-button-row>`

func TestRichResultsWithThumbnails(t *testing.T) {
	got := richResults(`<b>tool</b> & co`, richTestResults, true, richTestCallback)

	want := richTestHeader + `<img src="tg://photo?id=thumb0"/>` + richTestFirst + "<hr/>" + richTestSecond
	if got.HTML != want {
		t.Fatalf("unexpected html:\n got %s\nwant %s", got.HTML, want)
	}
	if len(got.Media) != 1 || got.Media[0].ID != "thumb0" || got.Media[0].Media.Media != "https://i.ytimg.com/vi/a/mqdefault.jpg" {
		t.Fatalf("unexpected media: %#v", got.Media)
	}
}

func TestRichResultsWithoutThumbnails(t *testing.T) {
	got := richResults(`<b>tool</b> & co`, richTestResults, false, richTestCallback)

	if want := richTestHeader + richTestFirst + "<p>&nbsp;</p>" + richTestSecond; got.HTML != want {
		t.Fatalf("unexpected html:\n got %s\nwant %s", got.HTML, want)
	}
	if strings.Contains(got.HTML, "<img") || len(got.Media) != 0 {
		t.Fatalf("expected no thumbnails, got %#v", got)
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
