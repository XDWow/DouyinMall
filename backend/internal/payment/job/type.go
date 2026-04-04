package job

type Job interface { // 瀹氭椂浠诲姟锛堝悓姝ヨ秴鏃惰鍗曪級
	Run() error
	Name() string
}


