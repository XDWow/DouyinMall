package support

import (
	"fmt"

	"github.com/cloudwego/eino/schema"
)

func FallbackAnswerFromDocuments(documents []*schema.Document) string {
	if len(documents) > 0 && documents[0] != nil {
		doc := documents[0]
		return fmt.Sprintf(
			"根据知识库《%s》，%s",
			FirstNonEmpty(DocumentTitle(doc), "参考资料"),
			DocumentSnippet(doc, 180),
		)
	}
	return "我暂时还缺少足够信息，请稍后重试或转人工处理。"
}
