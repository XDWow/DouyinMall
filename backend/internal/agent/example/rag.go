package main

import (
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
    "net/url"
    "strings"
    "time"
)

var KnowledgeBaseDomain = "api-knowledgebase.mlp.cn-beijing.volces.com" // 鐭ヨ瘑搴撳煙鍚?
var ServiceChatPath = "/api/knowledge/service/chat"                     // 鏀寔鐭ヨ瘑鏈嶅姟鐨勭煡璇嗗簱妫€绱㈡帴鍙?
var APIKey = "your api key"                                             // 鐢ㄤ簬鐭ヨ瘑鏈嶅姟閴存潈鐨刟pikey
var ServiceResourceID = "kb-service-a1a9f641ee516876"                   // 鎮ㄥ湪骞冲彴涓婂垱寤虹殑鐭ヨ瘑鏈嶅姟ID

type ServiceChatRequest struct {
    ServiceResourceID string         `json:"service_resource_id,omitempty"` //瑕佹绱㈢殑鐭ヨ瘑鏈嶅姟ID
    Stream            bool           `json:"stream"`                        // 浠呴拡瀵圭敓鎴愮被鍨嬬殑鐭ヨ瘑鏈嶅姟鐢熸晥锛岄粯璁や负娴佸紡杩斿洖锛宖alse鍒欎负闈炴祦寮忚繑鍥?
    Messages          []MessageParam `json:"messages"`                      // 澶氳疆瀵硅瘽淇℃伅Message鏁扮粍锛屾嫾鎺ョ殑澶氳疆瀵硅瘽message鐨剅ole椤哄簭濡備笅锛歔user, assistant, user...]锛屾渶鍚庝竴涓厓绱犻渶淇濊瘉鏄綋鍓嶈疆娆℃彁闂紝瑙掕壊涓簎ser
    QueryParam        QueryParamInfo `json:"query_param,omitempty"`         // 妫€绱㈤檮鍔犺繃婊ゆ潯浠讹紝鍦ㄥ垱寤虹煡璇嗘湇鍔℃椂濡傛灉鎮ㄤ篃閰嶇疆浜嗚繃婊ゆ潯浠讹紝閭ｄ箞鍜岃闄勫姞鏉′欢涓€璧风敓鏁堬紝閫昏緫涓篈ND
}

type QueryParamInfo struct {
    DocFilter interface{} `json:"doc_filter"`
}

type MessageParam struct {
    Role    string      `json:"role"`
    Content interface{} `json:"content"`
}

type ServiceChatResponse struct {
    Code    int64                              `json:"code"`
    Message string                             `json:"message,omitempty"`
    Data    *CollectionServiceChatResponseData `json:"data,omitempty"`
}

type CollectionServiceChatResponseData struct {
    CollectionSearchKnowledgeResponseData
    *CollectionChatCompletionResponseData
}

type CollectionSearchKnowledgeResponseData struct {
    Count        int32                           `json:"count"`
    RewriteQuery string                          `json:"rewrite_query,omitempty"`
    TokenUsage   *TotalTokenUsage                `json:"token_usage,omitempty"`
    ResultList   []*CollectionSearchResponseItem `json:"result_list,omitempty"`
}

// 妫€绱㈡帴鍙ｅ悇涓樁娈垫ā鍨嬭皟鐢ㄩ噺璇︽儏锛岃缁嗕粙缁嶈瀹樻柟鏂囨。
type TotalTokenUsage struct {
    EmbeddingUsage *ModelTokenUsage `json:"embedding_token_usage,omitempty"`
    RerankUsage    *int64           `json:"rerank_token_usage,omitempty"`
    LLMUsage       *ModelTokenUsage `json:"llm_token_usage,omitempty"`
    RewriteUsage   *ModelTokenUsage `json:"rewrite_token_usage,omitempty"`
}

// 妫€绱㈡帴鍙ｈ繑鍥炲垏鐗囩殑璇︽儏锛岃缁嗕粙缁嶈瀹樻柟鏂囨。
type CollectionSearchResponseItem struct {
    Id                  string                              `json:"id"`
    Content             string                              `json:"content"`
    MdContent           string                              `json:"md_content,omitempty"`
    Score               float64                             `json:"score"`
    PointId             string                              `json:"point_id"`
    OriginText          string                              `json:"origin_text,omitempty"`
    OriginalQuestion    string                              `json:"original_question,omitempty"`
    ChunkTitle          string                              `json:"chunk_title,omitempty"`
    ChunkId             int                                 `json:"chunk_id"`
    ProcessTime         int64                               `json:"process_time"`
    RerankScore         float64                             `json:"rerank_score,omitempty"`
    DocInfo             CollectionSearchResponseItemDocInfo `json:"doc_info,omitempty"`
    RecallPosition      int32                               `json:"recall_position"`
    RerankPosition      int32                               `json:"rerank_position,omitempty"`
    ChunkType           string                              `json:"chunk_type,omitempty"`
    ChunkSource         string                              `json:"chunk_source,omitempty"`
    UpdateTime          int64                               `json:"update_time"`
    ChunkAttachmentList []ChunkAttachment                   `json:"chunk_attachment,omitempty"`
    TableChunkFields    []PointTableChunkField              `json:"table_chunk_fields,omitempty"`
    OriginalCoordinate  *ChunkPositions                     `json:"original_coordinate,omitempty"`
}

type CollectionSearchResponseItemDocInfo struct {
    Docid      string `json:"doc_id"`
    DocName    string `json:"doc_name"`
    CreateTime int64  `json:"create_time"`
    DocType    string `json:"doc_type"`
    DocMeta    string `json:"doc_meta,omitempty"`
    Source     string `json:"source"`
    Title      string `json:"title,omitempty"`
}

type ChunkAttachment struct {
    UUID    string `json:"uuid,omitempty"`
    Caption string `json:"caption"`
    Type    string `json:"type"`
    Link    string `json:"link,omitempty"`
}

type PointTableChunkField struct {
    FieldName  string      `json:"field_name"`
    FieldValue interface{} `json:"field_value"`
}

type ChunkPositions struct {
    PageNo []int       `json:"page_no"`
    BBox   [][]float64 `json:"bbox"`
}

type CollectionChatCompletionResponseData struct {
    GenerateAnswer   string  `json:"generated_answer"`
    ReasoningContent string  `json:"reasoning_content,omitempty"`
    Prompt           *string `json:"prompt,omitempty"`
    End              bool    `json:"end,omitempty"`
}

type ModelTokenUsage struct {
    PromptTokens     int64 `json:"prompt_tokens"`     // 璇锋眰鏂囨湰鐨勫垎璇嶆暟
    CompletionTokens int64 `json:"completion_tokens"` // 鐢熸垚鏂囨湰鐨勫垎璇嶆暟, 瀵硅瘽妯″瀷鎵嶆湁鍊? 鍏朵粬妯″瀷閮芥槸0
    TotalTokens      int64 `json:"total_tokens"`      // PromptTokens + CompletionTokens
}

// scanDoubleCRLF 鏄竴涓?bufio.SplitFunc锛岀敤浜庡垎闅?\r\n\r\n
func scanDoubleCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
    // 鏌ユ壘 \r\n\r\n 鍒嗛殧绗?
    if i := bytes.Index(data, []byte("\r\n\r\n")); i >= 0 {
       // 杩斿洖浣嶇疆鍚庣殑鍒嗛殧绗?
       return i + 4, data[0:i], nil
    }
    if atEOF && strings.Contains(string(data), "\"end\":true") {
       return len(data), data, nil
    }
    return 0, nil, nil
}

func PrepareRequest(method string, path string, body []byte) *http.Request {
    u := url.URL{
       Scheme: "http",
       Host:   KnowledgeBaseDomain,
       Path:   path,
    }
    req, _ := http.NewRequest(strings.ToUpper(method), u.String(), bytes.NewReader(body))
    req.Header.Add("Accept", "application/json")
    req.Header.Add("Content-Type", "application/json")
    req.Header.Add("Host", KnowledgeBaseDomain)
    req.Header.Add("Authorization", "Bearer "+APIKey)
    return req
}

func GenerateServiceChatReq(stream bool) *ServiceChatRequest {
    return &ServiceChatRequest{
       ServiceResourceID: ServiceResourceID,
       Stream:            stream,
       Messages: []MessageParam{
          // 褰搎uery涓虹函鏂囨湰鏃讹紝user鐨刢ontent涓簈uery鏂囨湰
          {
             Role:    "user",
             Content: "29鍏冨椁愮數璇濆崱",
          },
          // 褰搎uery鍖呭惈鍥剧墖鏃讹紝user鐨刢ontent涓簂ist缁撴瀯
          //{
          // Role:    "user",
          // Content: []map[string]interface{}{
          //    {
          //       "text": "29鍏冨椁愮數璇濆崱",
          //       "type": "text",
          //    },
          //    {
          //       "image_url": map[string]string{
          //          "url": "璇蜂紶鍏ュ彲璁块棶鐨勫浘鐗嘦RL鎴栬€匓ase64缂栫爜",
          //       },
          //       "type": "image_url",
          //    },
          // },
          //},
       },
       //QueryParam: QueryParamInfo{},
    }
}

// KnowledgeServiceChat 鐭ヨ瘑鏈嶅姟-闈炴祦寮忚繑鍥?妫€绱㈢被鍨嬬殑鐭ヨ瘑鏈嶅姟鎴栬€呯敓鎴愮被鍨嬬殑鐭ヨ瘑鏈嶅姟闈炴祦寮忎娇鐢ㄨ鍑芥暟)
func KnowledgeServiceChat(serviceChatReq *ServiceChatRequest) error {
    serviceChatReqBytes, _ := json.Marshal(serviceChatReq)
    req := PrepareRequest("POST", ServiceChatPath, serviceChatReqBytes)
    client := &http.Client{Timeout: 600 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
       fmt.Printf("璇锋眰澶辫触: %s\n", err.Error())
       return err
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
       return err
    }
    fmt.Printf("鏈璇锋眰杩斿洖淇℃伅: %s\n", string(body))

    var serviceChatResp *ServiceChatResponse
    err = json.Unmarshal(body, &serviceChatResp)
    if err != nil {
       return err
    }
    return nil
}

// KnowledgeServiceChatStream 鐢熸垚绫诲瀷鐭ヨ瘑鏈嶅姟-娴佸紡杩斿洖锛堢敓鎴愮被鍨嬬殑鐭ヨ瘑鏈嶅姟娴佸紡杩斿洖浣跨敤璇ュ嚱鏁帮級
func KnowledgeServiceChatStream(serviceChatReq *ServiceChatRequest) (err error) {
    chatCompletionReqParamsBytes, _ := json.Marshal(serviceChatReq)
    request := PrepareRequest("POST", ServiceChatPath, chatCompletionReqParamsBytes)
    client := &http.Client{
       Timeout: time.Second * 600,
    }
    request.Header.Set("Accept", "text/event-stream")
    resp, err := client.Do(request)
    if err != nil {
       fmt.Printf("璇锋眰澶辫触: %s\n", err.Error())
       return err
    }
    defer resp.Body.Close()
    // 璇诲彇娴佸紡杩斿洖
    scanner := bufio.NewScanner(resp.Body)
    // 鎸囧畾鍒嗛殧绗﹀嚱鏁?
    scanner.Split(scanDoubleCRLF)

    var answerBuilder strings.Builder
    var usage TotalTokenUsage

    buf := make([]byte, 0, 150*1024)
    scanner.Buffer(buf, 1500*1024) // 鍙互鎸夐渶璋冩暣scanner鐨勫ぇ灏?

    // 璇诲彇鏁版嵁
    for scanner.Scan() {
       streamLine := scanner.Text()
       fmt.Println(streamLine)
       if len(streamLine) < 5 {
          continue
       }
       streamJson := streamLine[5:]
       var serviceChatResponse ServiceChatResponse
       err := json.Unmarshal([]byte(streamJson), &serviceChatResponse)
       if err != nil {
          fmt.Printf("璇锋眰澶辫触: %s\n", err.Error())
          return err
       }
       if serviceChatResponse.Data.TokenUsage != nil {
          usage = *serviceChatResponse.Data.TokenUsage
       }
       if serviceChatResponse.Data.End {
          fmt.Println("娴佸紡杈撳嚭杩斿洖缁撴潫")
          break
       }
       answerBuilder.WriteString(serviceChatResponse.Data.GenerateAnswer)
    }

    if err := scanner.Err(); err != nil {
       fmt.Printf("璇锋眰澶辫触: %s\n", err.Error())
       return err
    }
    usageStr, _ := json.Marshal(usage)
    fmt.Printf("鏈璇锋眰Token浣跨敤鎯呭喌: %s\n", usageStr)
    fmt.Printf("LLM鍥炵瓟: %s\n", answerBuilder.String())
    return nil
}

func main() {
    // 浠ヤ笅涓や釜鍑芥暟鏍规嵁闇€瑕佷簩閫変竴
    //绾绱㈢被鍨嬬殑鐭ヨ瘑鏈嶅姟鎴栬€呯敓鎴愮被鍨嬬煡璇嗘湇鍔￠潪娴佸紡杩斿洖浣跨敤璇ュ嚱鏁?
    KnowledgeServiceChat(GenerateServiceChatReq(false))
    //鐢熸垚绫诲瀷鐨勭煡璇嗘湇鍔℃祦寮忚繑鍥?浣跨敤璇ュ嚱鏁?
    KnowledgeServiceChatStream(GenerateServiceChatReq(true))
}
