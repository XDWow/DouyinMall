package config

type Config struct {
	GRPC          GRPCConfig          `mapstructure:"grpc"`
	HTTP          HTTPConfig          `mapstructure:"http"`
	DB            DBConfig            `mapstructure:"db"`
	Redis         RedisConfig         `mapstructure:"redis"`
	Etcd          EtcdConfig          `mapstructure:"etcd"`
	MCP           MCPConfig           `mapstructure:"mcp"`
	Skill         SkillConfig         `mapstructure:"skill"`
	Tenant        TenantConfig        `mapstructure:"tenant"`
	FeatureFlags  FeatureFlagsConfig  `mapstructure:"feature_flags"`
	LLM           LLMConfig           `mapstructure:"llm"`
	Embedding     EmbeddingConfig     `mapstructure:"embedding"`
	KnowledgeBase KnowledgeBaseConfig `mapstructure:"knowledge_base"`
	Workflow      WorkflowConfig      `mapstructure:"workflow"`
	Observability ObservabilityConfig `mapstructure:"observability"`
}

type AgentConfig = Config

type GRPCConfig struct {
	Server GRPCServerConfig `mapstructure:"server"`
}

type GRPCServerConfig struct {
	Port int    `mapstructure:"port"`
	Name string `mapstructure:"name"`
}

type HTTPConfig struct {
	Addr                string `mapstructure:"addr"`
	Prefix              string `mapstructure:"prefix"`
	ReadTimeoutSeconds  int    `mapstructure:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `mapstructure:"write_timeout_seconds"`
	IdleTimeoutSeconds  int    `mapstructure:"idle_timeout_seconds"`
}

type DBConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type EtcdConfig struct {
	Endpoints []string `mapstructure:"endpoints"`
}

type MCPConfig struct {
	Servers []MCPServerConfig `mapstructure:"servers"`
}

type MCPServerConfig struct {
	Name           string `mapstructure:"name"`
	Endpoint       string `mapstructure:"endpoint"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
	Enabled        bool   `mapstructure:"enabled"`
}

type SkillConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Roots   []string `mapstructure:"roots"`
}

type TenantConfig struct {
	DefaultID string `mapstructure:"default_id"`
}

type FeatureFlagsConfig struct {
	OrderQuery          bool `mapstructure:"order_query"`
	ReturnPolicy        bool `mapstructure:"return_policy"`
	Inventory           bool `mapstructure:"inventory"`
	ProductInfo         bool `mapstructure:"product_info"`
	ReturnExchangeApply bool `mapstructure:"return_exchange_apply"`
}

type LLMConfig struct {
	BaseURL        string  `mapstructure:"base_url"`
	APIKey         string  `mapstructure:"api_key"`
	Model          string  `mapstructure:"model"`
	TimeoutSeconds int     `mapstructure:"timeout_seconds"`
	Temperature    float32 `mapstructure:"temperature"`
	MaxTokens      int     `mapstructure:"max_tokens"`
}

type EmbeddingConfig struct {
	BaseURL        string `mapstructure:"base_url"`
	APIKey         string `mapstructure:"api_key"`
	Model          string `mapstructure:"model"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
}

type KnowledgeBaseConfig struct {
	Scheme            string `mapstructure:"scheme"`
	Domain            string `mapstructure:"domain"`
	ServiceChatPath   string `mapstructure:"service_chat_path"`
	ServiceResourceID string `mapstructure:"service_resource_id"`
	APIKey            string `mapstructure:"api_key"`
	TimeoutSeconds    int    `mapstructure:"timeout_seconds"`
}

type WorkflowConfig struct {
	RateLimitPerMinute      int64    `mapstructure:"rate_limit_per_minute"`
	ConversationWindow      int      `mapstructure:"conversation_window"`
	ExactCacheTTLSeconds    int      `mapstructure:"exact_cache_ttl_seconds"`
	L0CacheTTLSeconds       int      `mapstructure:"l0_cache_ttl_seconds"`
	SemanticCacheTTLSeconds int      `mapstructure:"semantic_cache_ttl_seconds"`
	SemanticCacheScore      float64  `mapstructure:"semantic_cache_score"`
	SemanticCacheTopK       int      `mapstructure:"semantic_cache_top_k"`
	RetrieveTopK            int      `mapstructure:"retrieve_top_k"`
	RetrieveMinScore        float64  `mapstructure:"retrieve_min_score"`
	RerankTopK              int      `mapstructure:"rerank_top_k"`
	ToolParallelism         int      `mapstructure:"tool_parallelism"`
	ConfidenceThreshold     float64  `mapstructure:"confidence_threshold"`
	MaxAnswerTokens         int      `mapstructure:"max_answer_tokens"`
	StreamBuffer            int      `mapstructure:"stream_buffer"`
	CheckpointTTLSeconds    int      `mapstructure:"checkpoint_ttl_seconds"`
	InterruptBeforeNodes    []string `mapstructure:"interrupt_before_nodes"`
	InterruptAfterNodes     []string `mapstructure:"interrupt_after_nodes"`
}

type ObservabilityConfig struct {
	Log     LogConfig     `mapstructure:"log"`
	Metrics MetricsConfig `mapstructure:"metrics"`
	Trace   TraceConfig   `mapstructure:"trace"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

type MetricsConfig struct {
	Path string `mapstructure:"path"`
}

type TraceConfig struct {
	Enabled     bool    `mapstructure:"enabled"`
	ServiceName string  `mapstructure:"service_name"`
	Endpoint    string  `mapstructure:"endpoint"`
	Insecure    bool    `mapstructure:"insecure"`
	SampleRatio float64 `mapstructure:"sample_ratio"`
}
