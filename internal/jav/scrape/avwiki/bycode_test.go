package avwiki

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// The markup mirrors the actress list of a search result card: one linked name
// per work, plus unrelated archive links that must not be picked up.
const byCodeSample = `<html><body>
<header class="archive-header">
  <h2 class="archive-header-title"><a href="https://al.fanza.co.jp/?lurl=x">徹夜残業の…</a></h2>
  <ul class="post-meta clearfix">
    <li class="actress-name"><i class="fa fa-venus"></i><a href="https://av-wiki.net/av-actress/takeuchi-yuki/" rel="tag">竹内有紀</a></li>
    <li><i class="fa fa-clone"></i><a href="https://av-wiki.net/fanza-video/hon-naka/hmn/">本中 - HMN</a></li>
    <li><i class="fa fa-circle-o"></i>HMN-929</li>
  </ul>
</header>
<div class="read-more"><a href="https://av-wiki.net/hmn-929/">続きを読む</a></div>
</body></html>`

func TestCollectActressRefsReadsActressBlock(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(byCodeSample))
	if err != nil {
		t.Fatalf("parse sample: %v", err)
	}
	refs := collectActressRefs(doc, "https://av-wiki.net/?s=HMN-929&post_type=product")
	if len(refs) != 1 {
		t.Fatalf("expected 1 actress, got %+v", refs)
	}
	if refs[0].Name != "竹内有紀" {
		t.Fatalf("unexpected name %q", refs[0].Name)
	}
	if refs[0].ProfileURL != "https://av-wiki.net/av-actress/takeuchi-yuki/" {
		t.Fatalf("unexpected profile url %q", refs[0].ProfileURL)
	}
}

func TestCollectActressRefsDeduplicates(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(byCodeSample + byCodeSample))
	if err != nil {
		t.Fatalf("parse sample: %v", err)
	}
	refs := collectActressRefs(doc, "https://av-wiki.net/?s=HMN-929&post_type=product")
	if len(refs) != 1 {
		t.Fatalf("expected duplicates removed, got %+v", refs)
	}
}

func TestPageHTTPClientRaisesTimeout(t *testing.T) {
	client := New(nil)
	if got := client.pageHTTPClient().Timeout; got != byCodePageTimeout {
		t.Fatalf("nil client timeout = %v, want %v", got, byCodePageTimeout)
	}
}
