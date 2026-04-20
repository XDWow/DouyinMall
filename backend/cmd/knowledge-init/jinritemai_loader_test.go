package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/document"
)

func TestExtractJinritemaiArticleID(t *testing.T) {
	t.Parallel()

	articleID, err := extractJinritemaiArticleID("https://school.jinritemai.com/doudian/web/article/101835?foo=bar")
	if err != nil {
		t.Fatalf("extract article id failed: %v", err)
	}
	if articleID != "101835" {
		t.Fatalf("unexpected article id: %s", articleID)
	}
}

func TestExtractPlainTextFromJinritemaiContent(t *testing.T) {
	t.Parallel()

	raw := `{"deltas":{"0":{"ops":[{"attributes":{"lmkr":"1","heading":"h1"},"insert":"*"},{"insert":"七天无理由退货服务规范"},{"insert":"\n"},{"attributes":{"lmkr":"1","list":"bullet1"},"insert":"*"},{"insert":"商品保持完好"},{"insert":"\n"}]}}}`

	text := extractPlainTextFromJinritemaiContent(raw)
	if !strings.Contains(text, "七天无理由退货服务规范") {
		t.Fatalf("expected title in extracted text, got: %s", text)
	}
	if !strings.Contains(text, "- 商品保持完好") {
		t.Fatalf("expected bullet item in extracted text, got: %s", text)
	}
}

func TestJinritemaiArticleLoaderLoad(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/eschool/v2/library/article/detail" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("id"); got != "101835" {
			t.Fatalf("unexpected id query: %s", got)
		}
		if got := r.URL.Query().Get("graphId"); got != "312" {
			t.Fatalf("unexpected graphId query: %s", got)
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"article_info":{"article_id":"101835","name":"七天无理由退货服务规范","content":"{\"deltas\":{\"0\":{\"ops\":[{\"attributes\":{\"lmkr\":\"1\",\"heading\":\"h1\"},\"insert\":\"*\"},{\"insert\":\"七天无理由退货服务规范\"},{\"insert\":\"\\n\"},{\"insert\":\"商品完好标准\"},{\"insert\":\"\\n\"}]}}}","creator_name":"电商运营团队","tags":["退款","规则"]}}}`))
	}))
	defer server.Close()

	loader := &jinritemaiArticleLoader{
		client:  server.Client(),
		graphID: 312,
		baseURL: server.URL,
	}

	docs, err := loader.Load(context.Background(), document.Source{
		URI: "https://school.jinritemai.com/doudian/web/article/101835",
	})
	if err != nil {
		t.Fatalf("load article failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("unexpected docs length: %d", len(docs))
	}
	if docs[0].Content == "" {
		t.Fatalf("expected content to be populated")
	}
	if got := stringifyMetaValue(docs[0].MetaData["title"]); got != "七天无理由退货服务规范" {
		t.Fatalf("unexpected title metadata: %s", got)
	}
}
